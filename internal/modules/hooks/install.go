package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Plugins do Claude Code declaram hooks por convenção em
// <plugin>/hooks/hooks.json, e os comandos apontam para os próprios arquivos
// via ${CLAUDE_PLUGIN_ROOT} — a raiz do plugin, que só o Claude Code expande
// e só para plugin instalado por ele.
//
// Importar é, então: copiar da raiz do plugin as pastas que os comandos
// citam (preservando o layout, porque script costuma se localizar por
// caminho relativo à própria raiz), apontar a variável para a cópia e
// exportá-la, para o script que a lê por dentro continuar funcionando.
const (
	hooksDirName  = "hooks"
	hooksFile     = "hooks.json"
	pluginRootVar = "CLAUDE_PLUGIN_ROOT"
	pluginRoot    = "${" + pluginRootVar + "}"
	pluginRootSh  = "$" + pluginRootVar
	// maxDepth limita a varredura da origem: plugin/hooks/hooks.json é o
	// fundo esperado, e repositório grande não vira varredura infinita.
	maxDepth = 5
)

// Found é um hooks.json encontrado numa origem (repo clonado, pasta ou zip),
// candidato a importação. Os comandos ainda estão como o plugin escreveu.
type Found struct {
	Plugin      string       // nome do plugin (a pasta que contém hooks/)
	Root        string       // raiz do plugin: o que ${CLAUDE_PLUGIN_ROOT} significa
	Dir         string       // pasta hooks/ na origem
	Rel         string       // caminho relativo à raiz da origem
	Description string       // description do hooks.json
	Hooks       []agent.Hook // o que o arquivo declara
}

// Events devolve os eventos distintos do arquivo, para exibição.
func (f Found) Events() []string {
	seen := map[string]bool{}
	var out []string
	for _, h := range f.Hooks {
		if !seen[h.Event] {
			seen[h.Event] = true
			out = append(out, h.Event)
		}
	}
	return out
}

// DiscoverIn varre uma origem já materializada em disco procurando
// hooks/hooks.json. rootName nomeia o plugin cujo hooks/ está na raiz da
// origem — um clone vive num diretório temporário, e o nome dele não serve.
// Erro de leitura de um plugin não interrompe a varredura: o que não dá para
// ler simplesmente não aparece.
func DiscoverIn(root, rootName string) []Found {
	var out []Found
	rootClean := filepath.Clean(root)
	_ = filepath.WalkDir(rootClean, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(rootClean, path)
		if depth := len(strings.Split(filepath.ToSlash(rel), "/")); rel != "." && depth > maxDepth {
			return fs.SkipDir
		}
		if name := d.Name(); rel != "." && strings.HasPrefix(name, ".") && name != "." {
			return fs.SkipDir // .git, .github…
		}
		if d.Name() != hooksDirName {
			return nil
		}
		file := filepath.Join(path, hooksFile)
		hooks, err := agent.ReadHookFile(file)
		if err != nil || len(hooks) == 0 {
			return fs.SkipDir
		}
		pluginRootDir := filepath.Dir(path)
		plugin := filepath.Base(pluginRootDir)
		if pluginRootDir == rootClean && rootName != "" {
			plugin = rootName // hooks/ na raiz da origem: o dir é temporário
		}
		relDir, _ := filepath.Rel(rootClean, path)
		out = append(out, Found{
			Plugin:      plugin,
			Root:        pluginRootDir,
			Dir:         path,
			Rel:         filepath.ToSlash(relDir),
			Description: hookFileDescription(file),
			Hooks:       hooks,
		})
		return fs.SkipDir
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Plugin < out[j].Plugin })
	return out
}

// hookFileDescription lê o campo "description" do hooks.json, se houver.
func hookFileDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		Description string `json:"description"`
	}
	_ = json.Unmarshal(data, &doc)
	return doc.Description
}

// Import copia as pastas referenciadas e grava uma entrada por plugin.
// Destinos existentes são recusados antes de copiar qualquer arquivo.
func Import(paths core.Paths, f Found, source string) (names []string, err error) {
	if !nameRe.MatchString(f.Plugin) || len([]rune(f.Plugin)) > maxNameLen {
		return nil, fmt.Errorf("invalid plugin name: %q", f.Plugin)
	}
	if _, err := os.Lstat(filepath.Join(paths.HooksDir(), f.Plugin+".json")); !os.IsNotExist(err) {
		return nil, fmt.Errorf("hook %q already exists or cannot be accessed", f.Plugin)
	}
	dst := filepath.Join(paths.HooksDir(), f.Plugin)
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		return nil, fmt.Errorf("hooks from %q are already in the library (remove them before reinstalling)", f.Plugin)
	}
	entry, err := libraryEntry(f, dst, source)
	if err != nil {
		return nil, err
	}
	for _, rel := range referencedPaths(f) {
		src := filepath.Join(f.Root, rel)
		if _, err := os.Stat(src); err != nil {
			_ = os.RemoveAll(dst)
			return nil, fmt.Errorf("%s: the commands reference %q, which does not exist in the source", f.Plugin, rel)
		}
		if err := copyTree(src, filepath.Join(dst, rel)); err != nil {
			_ = os.RemoveAll(dst)
			return nil, fmt.Errorf("copying %s from %q: %w", rel, f.Plugin, err)
		}
	}
	svc := &Service{dir: paths.HooksDir()}
	if err := svc.Save(entry); err != nil {
		_ = os.RemoveAll(dst) // import é tudo ou nada
		return nil, fmt.Errorf("importing %q: %w", f.Plugin, err)
	}
	return []string{entry.Name}, nil
}

// libraryEntry traduz o hooks.json do plugin numa entrada da biblioteca, com
// os comandos já apontando para os scripts copiados. Um plugin = uma
// entrada: o pacote liga e desliga inteiro.
func libraryEntry(f Found, dst, source string) (Hook, error) {
	entry := Hook{
		Name:        f.Plugin,
		Description: f.Description,
		Source:      strings.TrimSpace(source + " · " + f.Plugin),
		Files:       dst,
	}
	for _, h := range f.Hooks {
		command, err := rewriteCommand(h.Command, dst)
		if err != nil {
			return Hook{}, fmt.Errorf("%s (%s): %w", f.Plugin, h.Event, err)
		}
		h.Command = command
		// Plugin real repete o mesmo comando em vários grupos (matchers
		// diferentes que viram a mesma tripla). Identidade repetida no pacote
		// bagunçaria a contagem de instalados, então entra uma vez só.
		duplicate := false
		for _, existing := range entry.Hooks {
			if existing.Same(h) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			entry.Hooks = append(entry.Hooks, h)
		}
	}
	return entry, nil
}

// referencedPaths devolve, sem repetir, as pastas de primeiro nível da raiz
// do plugin citadas pelos comandos (sempre incluindo hooks/, onde mora o
// próprio hooks.json). É o que é copiado: o plugin inteiro costuma trazer
// docs, testes e as próprias skills, que já têm biblioteca própria.
func referencedPaths(f Found) []string {
	seen := map[string]bool{hooksDirName: true}
	out := []string{hooksDirName}
	for _, h := range f.Hooks {
		for _, rel := range rootRefs(h.Command) {
			if !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

// rootRefs extrai o primeiro segmento de cada ${CLAUDE_PLUGIN_ROOT}/<seg>/…
// do comando.
func rootRefs(command string) []string {
	var out []string
	for _, ref := range []string{pluginRoot, pluginRootSh} {
		rest := command
		for {
			i := strings.Index(rest, ref+"/")
			if i < 0 {
				break
			}
			rest = rest[i+len(ref)+1:]
			seg := rest
			if j := strings.IndexAny(seg, `/"' 	`); j >= 0 {
				seg = seg[:j]
			}
			if seg != "" && seg != "." && seg != ".." {
				out = append(out, seg)
			}
		}
	}
	return out
}

// rewriteCommand define a raiz antes de o shell expandir o comando original.
// Preserva as aspas do autor e não insere caminhos dentro de código shell.
//
// A forma ${CLAUDE_PLUGIN_ROOT} vira $CLAUDE_PLUGIN_ROOT: o Claude Code
// recusa, no settings.json, todo comando que contenha o texto literal
// "${CLAUDE_PLUGIN_ROOT}" ("the hook is not associated with a plugin"),
// mesmo que o próprio comando exporte a variável. Sem chaves o shell expande
// igual, desde que o caractere seguinte não continue o nome da variável.
func rewriteCommand(command, dst string) (string, error) {
	if !strings.Contains(command, pluginRootVar) {
		return command, nil
	}
	command, err := unbracePluginRoot(command)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("export %s=%s; %s", pluginRootVar, shellQuote(dst), command), nil
}

// stripRootExport tira o "export CLAUDE_PLUGIN_ROOT='…'; " que rewriteCommand
// põe na frente, devolvendo o comando do plugin.
func stripRootExport(command string) string {
	if !strings.HasPrefix(command, "export "+pluginRootVar+"=") {
		return command
	}
	if i := strings.Index(command, "; "); i >= 0 {
		return command[i+2:]
	}
	return command
}

// unbracePluginRoot troca ${CLAUDE_PLUGIN_ROOT} por $CLAUDE_PLUGIN_ROOT.
// Quando o nome seria engolido pelo que vem depois (${CLAUDE_PLUGIN_ROOT}x),
// recusa em vez de mudar o significado do comando.
func unbracePluginRoot(command string) (string, error) {
	var b strings.Builder
	rest := command
	for {
		i := strings.Index(rest, pluginRoot)
		if i < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		after := rest[i+len(pluginRoot):]
		if after != "" && isNameByte(after[0]) {
			return "", fmt.Errorf("command uses %s glued to a name (%q): Claude Code does not accept that form outside a plugin", pluginRoot, command)
		}
		b.WriteString(rest[:i] + pluginRootSh)
		rest = after
	}
}

func isNameByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// shellQuote protege um caminho para uso numa linha de shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// copyTree copia uma árvore de arquivos preservando o bit de execução (os
// scripts do hook precisam dele).
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil // symlink e afins não entram na biblioteca
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// RepairImported corrige entradas importadas antes de rewriteCommand trocar
// ${CLAUDE_PLUGIN_ROOT} por $CLAUDE_PLUGIN_ROOT — o Claude Code recusava
// esses comandos. Troca cada comando velho pelo novo onde estiver instalado
// (a identidade é o comando, então é remover e adicionar) e regrava a
// entrada. Devolve os nomes reparados; sem nada a reparar, não toca em
// arquivo nenhum.
func (s *Service) RepairImported() ([]string, error) {
	lib, _ := s.Library()
	var repaired, errs []string
	for _, entry := range lib {
		var olds, news []agent.Hook
		for i, h := range entry.Hooks {
			prefix := pluginRootVar + "="
			if !strings.HasPrefix(h.Command, "export "+prefix) || !strings.Contains(h.Command, pluginRoot) {
				continue
			}
			fixed, err := unbracePluginRoot(h.Command)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", entry.Name, err))
				continue
			}
			olds = append(olds, h)
			h.Command = fixed
			news = append(news, h)
			entry.Hooks[i] = h
		}
		if len(olds) == 0 {
			continue
		}
		if err := s.swapInstalled(olds, news); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entry.Name, err))
			continue // a entrada velha continua batendo com o que ficou instalado
		}
		if err := s.Save(entry); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entry.Name, err))
			continue
		}
		repaired = append(repaired, entry.Name)
	}
	if len(errs) > 0 {
		return repaired, fmt.Errorf("reparando hooks: %s", strings.Join(errs, "; "))
	}
	return repaired, nil
}

// swapInstalled troca, em cada agente onde o comando velho está instalado,
// pelo novo.
func (s *Service) swapInstalled(olds, news []agent.Hook) error {
	for _, ad := range s.adapters {
		host, ok := ad.(agent.HooksHost)
		if !ok {
			continue
		}
		installed, err := host.ReadHooks()
		if err != nil {
			continue // arquivo ilegível: o Status já mostra o erro
		}
		for i, old := range olds {
			if !containsHook(installed, old) {
				continue
			}
			if err := host.AddHook(news[i], s.backupsDir); err != nil {
				return fmt.Errorf("%s: %w", ad.ID(), err)
			}
			if err := host.RemoveHook(old, s.backupsDir); err != nil {
				return fmt.Errorf("%s: %w", ad.ID(), err)
			}
		}
	}
	return nil
}
