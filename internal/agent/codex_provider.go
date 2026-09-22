package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// O Codex guarda a config num TOML que também carrega estado que não é nosso
// (`[projects.*]`, `[hooks.state.*]` com hashes de confiança). Reserializar o
// arquivo com uma lib de TOML reescreveria tudo isso, então o lazyagents
// edita só blocos delimitados por estes marcadores e copia o resto linha a
// linha — sem dependência nova e sem tocar no que é do usuário.
const (
	codexBlockStart = "# lazyagents — início do bloco gerenciado (não editar à mão)"
	codexBlockEnd   = "# lazyagents — fim do bloco gerenciado"
	// codexProviderID é o id do provider table que o lazyagents gerencia. Um
	// só: os perfis vivem em providers.json, o config.toml só guarda o ativo.
	codexProviderID = "lazyagents"
	// codexPrevModel guarda, dentro do bloco, o `model` que o usuário tinha.
	// O perfil precisa trocar essa chave (TOML não aceita a mesma chave duas
	// vezes), então o valor antigo viaja como comentário e volta no Clear.
	codexPrevModel = "# lazyagents: model anterior = "
)

// ProviderFile é o config.toml do Codex.
func (c *Codex) ProviderFile() string { return filepath.Join(c.configDir(), "config.toml") }

func (c *Codex) ReadProvider() (ProviderProfile, bool, error) {
	data, err := os.ReadFile(c.ProviderFile())
	if err != nil {
		if os.IsNotExist(err) {
			return ProviderProfile{}, false, nil
		}
		return ProviderProfile{}, false, fmt.Errorf("lendo %s: %w", c.ProviderFile(), err)
	}
	top, providers := parseCodexTOML(string(data))
	id := top["model_provider"]
	if id == "" {
		return ProviderProfile{}, false, nil
	}
	tbl := providers[id]
	p := ProviderProfile{
		Name:    id,
		BaseURL: tbl["base_url"],
		Model:   top["model"],
		EnvKey:  tbl["env_key"],
		WireAPI: tbl["wire_api"],
	}
	if n := tbl["name"]; n != "" {
		p.Name = n
	}
	return p, true, nil
}

func (c *Codex) ApplyProvider(p ProviderProfile, backupsDir string) error {
	if p.BaseURL == "" {
		return fmt.Errorf("o perfil precisa de baseUrl (vira base_url em [model_providers])")
	}
	if p.Token != "" && p.EnvKey == "" {
		// O Codex não aceita token no config.toml: ele lê a variável
		// apontada por env_key. Falhar é melhor que gravar um perfil que
		// autentica sem o token que o usuário acha que aplicou.
		return fmt.Errorf("o Codex lê o token de uma variável de ambiente — defina envKey no perfil e exporte a variável")
	}

	top := []string{fmt.Sprintf("model_provider = %s", tomlString(codexProviderID))}
	if p.Model != "" {
		top = append(top, fmt.Sprintf("model = %s", tomlString(p.Model)))
	}
	table := []string{fmt.Sprintf("[model_providers.%s]", codexProviderID)}
	name := p.Name
	if name == "" {
		name = codexProviderID
	}
	table = append(table, fmt.Sprintf("name = %s", tomlString(name)))
	table = append(table, fmt.Sprintf("base_url = %s", tomlString(p.BaseURL)))
	if p.EnvKey != "" {
		table = append(table, fmt.Sprintf("env_key = %s", tomlString(p.EnvKey)))
	}
	if p.WireAPI != "" {
		table = append(table, fmt.Sprintf("wire_api = %s", tomlString(p.WireAPI)))
	}
	return c.writeTOML(backupsDir, top, table, p.Model != "")
}

func (c *Codex) ClearProvider(backupsDir string) error {
	return c.writeTOML(backupsDir, nil, nil, false)
}

// writeTOML regrava o config.toml com os blocos gerenciados trocados: o bloco
// de chaves de topo entra antes da primeira tabela (chave solta depois de um
// [header] pertenceria àquela tabela) e o bloco da tabela do provider vai
// para o fim. Blocos vazios sobram removidos.
func (c *Codex) writeTOML(backupsDir string, top, table []string, setsModel bool) error {
	path := c.ProviderFile()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("lendo %s: %w", path, err)
	}
	all := splitLines(string(data))
	prevModel := codexPrevModelOf(all)
	lines := stripCodexBlocks(all)
	if setsModel {
		var userModel string
		lines, userModel = takeTopModel(lines)
		if prevModel == "" {
			prevModel = userModel
		}
		if prevModel != "" {
			top = append(top, codexPrevModel+tomlString(prevModel))
		}
	}
	if !setsModel && prevModel != "" {
		// Clear: devolve o model do usuário, como chave solta no topo (não
		// dá para saber a linha exata de onde ele saiu, e topo é onde chave
		// de topo vale).
		lines = append([]string{fmt.Sprintf("model = %s", tomlString(prevModel))}, lines...)
	}

	if len(top) > 0 {
		at := len(lines)
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "[") {
				at = i
				break
			}
		}
		block := wrapCodexBlock(top)
		lines = append(lines[:at], append(block, lines[at:]...)...)
	}
	if len(table) > 0 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, wrapCodexBlock(table)...)
	}

	out := strings.Join(lines, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	if backupsDir != "" {
		if _, err := fsutil.Backup(path, backupsDir); err != nil {
			return err
		}
		_ = fsutil.RotateBackups(backupsDir, filepath.Base(path)+".", settingsBackups)
	}
	if err := fsutil.WriteAtomic(path, []byte(out), perm); err != nil {
		return fmt.Errorf("gravando %s: %w", path, err)
	}
	return nil
}

// codexPrevModelOf lê o `model` original guardado no comentário do bloco.
func codexPrevModelOf(lines []string) string {
	for _, l := range lines {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(l), codexPrevModel); ok {
			if v, ok := tomlUnquote(rest); ok {
				return v
			}
		}
	}
	return ""
}

// takeTopModel remove a chave `model` de topo (fora de tabela) e devolve o
// valor que ela tinha: o bloco gerenciado vai declarar a sua, e repetir a
// chave quebraria o TOML.
func takeTopModel(lines []string) ([]string, string) {
	out := make([]string, 0, len(lines))
	value := ""
	inTable := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			inTable = true
		}
		if !inTable && strings.HasPrefix(trimmed, "model") {
			if key, raw, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == "model" {
				if v, ok := tomlUnquote(strings.TrimSpace(raw)); ok {
					value = v
					continue
				}
			}
		}
		out = append(out, l)
	}
	return out, value
}

func wrapCodexBlock(body []string) []string {
	block := append([]string{codexBlockStart}, body...)
	return append(block, codexBlockEnd, "")
}

// stripCodexBlocks remove todos os blocos gerenciados (marcador de início até
// o de fim, inclusive). Bloco sem fim consome até o final do arquivo — é o
// único caso em que ele existe, e deixar metade seria pior. A linha em branco
// que o próprio wrapCodexBlock escreveu depois do bloco também sai, para que
// aplicar e limpar devolvam o arquivo exatamente como estava.
func stripCodexBlocks(lines []string) []string {
	out := make([]string, 0, len(lines))
	inBlock, justClosed := false, false
	for _, l := range lines {
		switch strings.TrimSpace(l) {
		case codexBlockStart:
			inBlock, justClosed = true, false
			continue
		case codexBlockEnd:
			inBlock, justClosed = false, true
			continue
		}
		if inBlock {
			continue
		}
		if justClosed {
			justClosed = false
			if strings.TrimSpace(l) == "" {
				continue
			}
		}
		out = append(out, l)
	}
	// A linha em branco que fechava o bloco não precisa sobrar no fim.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// parseCodexTOML extrai só o que o módulo de providers precisa: as chaves
// soltas de topo e as tabelas [model_providers.<id>], sempre com valores de
// string. Não é um parser de TOML — qualquer outra construção é ignorada, o
// que é seguro porque a escrita nunca depende dele.
func parseCodexTOML(data string) (top map[string]string, providers map[string]map[string]string) {
	top = map[string]string{}
	providers = map[string]map[string]string{}
	table := ""
	for _, raw := range splitLines(data) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			table = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value, ok = tomlUnquote(strings.TrimSpace(value))
		if !ok {
			continue
		}
		switch {
		case table == "":
			top[key] = value
		case strings.HasPrefix(table, "model_providers."):
			id := strings.Trim(strings.TrimPrefix(table, "model_providers."), `"`)
			if providers[id] == nil {
				providers[id] = map[string]string{}
			}
			providers[id][key] = value
		}
	}
	return top, providers
}

// tomlString escreve uma basic string TOML.
func tomlString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}

// tomlUnquote lê uma basic string TOML (com comentário de linha opcional
// depois). ok=false para qualquer outro tipo de valor.
func tomlUnquote(s string) (string, bool) {
	if !strings.HasPrefix(s, `"`) {
		return "", false
	}
	var b strings.Builder
	escaped := false
	for _, r := range s[1:] {
		if escaped {
			switch r {
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			default:
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '"':
			return b.String(), true
		default:
			b.WriteRune(r)
		}
	}
	return "", false
}
