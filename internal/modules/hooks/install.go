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

// Import copia os scripts do plugin para <DataDir>/hooks/<plugin>/ e grava
// uma entrada de biblioteca por hook declarado. source é a origem legível
// (ex.: "usuario/repo"), guardada na entrada.
//
// Comando que aponta para fora de hooks/ é recusado: só essa pasta é
// copiada, e um hook que quebraria em silêncio na hora do evento é pior que
// um import que falha agora.
func Import(paths core.Paths, f Found, source string) (names []string, err error) {
	dst := filepath.Join(paths.HooksDir(), f.Plugin)
	if _, err := os.Stat(dst); err == nil {
		return nil, fmt.Errorf("hooks de %q já estão na biblioteca (remova antes de reinstalar)", f.Plugin)
	}
	entry, err := libraryEntry(f, dst, source)
	if err != nil {
		return nil, err
	}
	for _, rel := range referencedPaths(f) {
		src := filepath.Join(f.Root, rel)
		if _, err := os.Stat(src); err != nil {
			_ = os.RemoveAll(dst)
			return nil, fmt.Errorf("%s: os comandos citam %q, que não existe na origem", f.Plugin, rel)
		}
		if err := copyTree(src, filepath.Join(dst, rel)); err != nil {
			_ = os.RemoveAll(dst)
			return nil, fmt.Errorf("copiando %s de %q: %w", rel, f.Plugin, err)
		}
	}
	svc := &Service{dir: paths.HooksDir()}
	if err := svc.Save(entry); err != nil {
		_ = os.RemoveAll(dst) // import é tudo ou nada
		return nil, fmt.Errorf("importando %q: %w", f.Plugin, err)
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
	if len([]rune(entry.Name)) > maxNameLen {
		entry.Name = string([]rune(entry.Name)[:maxNameLen])
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

// rewriteCommand aponta ${CLAUDE_PLUGIN_ROOT} para a cópia e exporta a
// variável: o comando é expandido pelo shell antes da atribuição valer, então
// os caminhos são trocados no texto E a variável é definida para os scripts
// que a leem por dentro.
func rewriteCommand(command, dst string) (string, error) {
	if !strings.Contains(command, pluginRootVar) {
		return command, nil
	}
	for _, ref := range []string{pluginRoot, pluginRootSh} {
		command = strings.ReplaceAll(command, ref, dst)
	}
	return fmt.Sprintf("%s=%s %s", pluginRootVar, shellQuote(dst), command), nil
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
