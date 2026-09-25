package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// O Codex guarda a config num TOML que também carrega estado que não é nosso
// (`[projects.*]`, `[hooks.state.*]` com hashes de confiança). Reserializar o
// arquivo com uma lib de TOML reescreveria tudo isso, então o lazyagents
// edita só blocos delimitados por estes marcadores e copia o resto linha a
// linha — sem dependência nova e sem tocar no que é do usuário.
const (
	codexBlockStart = "# lazyagents — managed block start (do not edit by hand)"
	codexBlockEnd   = "# lazyagents — managed block end"
	// codexProviderID é o id do provider table que o lazyagents gerencia. Um
	// só: os perfis vivem em providers.json, o config.toml só guarda o ativo.
	codexProviderID = "lazyagents"
	// codexPrevModel guarda, dentro do bloco, o `model` que o usuário tinha.
	// O perfil precisa trocar essa chave (TOML não aceita a mesma chave duas
	// vezes), então o valor antigo viaja como comentário e volta no Clear.
	codexPrevModel    = "# lazyagents: previous model = "
	codexPrevProvider = "# lazyagents: previous provider = "
)

// Markers written before the English migration. Configs in the wild still
// carry them, so they are recognized on read; the next write replaces them
// with the English ones.
const (
	codexLegacyBlockStart   = "# lazyagents — início do bloco gerenciado (não editar à mão)" // check-english:allow
	codexLegacyBlockEnd     = "# lazyagents — fim do bloco gerenciado"
	codexLegacyPrevModel    = "# lazyagents: model anterior = "
	codexLegacyPrevProvider = "# lazyagents: provider anterior = "
)

// codexMarker classifies a trimmed line as a managed block start or end, in
// either the current or the legacy spelling.
func codexMarker(trimmed string) (start, end bool) {
	switch trimmed {
	case codexBlockStart, codexLegacyBlockStart:
		return true, false
	case codexBlockEnd, codexLegacyBlockEnd:
		return false, true
	}
	return false, false
}

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
	if p.WireAPI != "" && p.WireAPI != "responses" {
		return fmt.Errorf("Codex suporta apenas wireApi responses")
	}
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
	depth := 0
	for _, line := range all {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") && (strings.Contains(trimmed, `"""`) || strings.Contains(trimmed, "'''")) {
			return fmt.Errorf("config TOML com string multilinha: edição automática não suportada; arquivo preservado")
		}
		switch start, end := codexMarker(trimmed); {
		case start:
			if depth != 0 {
				return fmt.Errorf("blocos lazyagents aninhados; config preservada")
			}
			depth++
		case end:
			if depth != 1 {
				return fmt.Errorf("fim de bloco lazyagents sem início; config preservada")
			}
			depth--
		}
	}
	if depth != 0 {
		return fmt.Errorf("bloco lazyagents sem fim; config preservada")
	}
	prevModel := codexPrevValueOf(all, codexPrevModel, codexLegacyPrevModel)
	prevProvider := codexPrevValueOf(all, codexPrevProvider, codexLegacyPrevProvider)
	lines := stripCodexBlocks(all)
	if len(top) > 0 {
		_, providers := parseCodexTOML(strings.Join(lines, "\n"))
		if _, exists := providers[codexProviderID]; exists {
			return fmt.Errorf("tabela model_providers.lazyagents já existe fora do bloco gerenciado")
		}
		var original string
		lines, original, err = takeTopKey(lines, "model_provider")
		if err != nil {
			return err
		}
		if prevProvider == "" {
			prevProvider = original
		}
		if prevProvider != "" {
			top = append(top, codexPrevProvider+tomlString(prevProvider))
		}
	} else if prevProvider != "" {
		lines = append([]string{"model_provider = " + tomlString(prevProvider)}, lines...)
	}
	if setsModel {
		var userModel string
		lines, userModel, err = takeTopKey(lines, "model")
		if err != nil {
			return err
		}
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

// codexPrevValueOf reads the original value kept in a block comment under
// any of the given prefixes (current and legacy spelling).
func codexPrevValueOf(lines []string, prefixes ...string) string {
	for _, l := range lines {
		for _, prefix := range prefixes {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(l), prefix); ok {
				if v, ok := tomlUnquote(rest); ok {
					return v
				}
			}
		}
	}
	return ""
}

// takeTopKey remove uma chave de topo (fora de tabela) e devolve o
// valor que ela tinha: o bloco gerenciado vai declarar a sua, e repetir a
// chave quebraria o TOML.
func takeTopKey(lines []string, wanted string) ([]string, string, error) {
	out := make([]string, 0, len(lines))
	value := ""
	inTable := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			inTable = true
		}
		if !inTable {
			if key, raw, ok := strings.Cut(trimmed, "="); ok && strings.Trim(strings.TrimSpace(key), `"'`) == wanted {
				if v, ok := tomlUnquote(strings.TrimSpace(raw)); ok {
					value = v
					continue
				}
				return nil, "", fmt.Errorf("valor de %s não suportado; config preservada", wanted)
			}
		}
		out = append(out, l)
	}
	return out, value, nil
}

func wrapCodexBlock(body []string) []string {
	block := append([]string{codexBlockStart}, body...)
	return append(block, codexBlockEnd, "")
}

// stripCodexBlocks remove todos os blocos gerenciados (marcador de início até
// o de fim, inclusive), após validação em writeTOML. A linha em branco
// que o próprio wrapCodexBlock escreveu depois do bloco também sai, para que
// aplicar e limpar devolvam o arquivo exatamente como estava.
func stripCodexBlocks(lines []string) []string {
	out := make([]string, 0, len(lines))
	inBlock, justClosed := false, false
	for _, l := range lines {
		switch start, end := codexMarker(strings.TrimSpace(l)); {
		case start:
			inBlock, justClosed = true, false
			continue
		case end:
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
			if end := strings.IndexByte(line, ']'); end >= 0 {
				table = strings.Trim(line[:end+1], "[]")
				if strings.HasPrefix(table, "model_providers.") {
					id := strings.Trim(strings.TrimPrefix(table, "model_providers."), `"'`)
					if providers[id] == nil {
						providers[id] = map[string]string{}
					}
				}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.Trim(strings.TrimSpace(key), `"'`)
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
	data, _ := json.Marshal(s)
	return string(data)
}

// tomlUnquote lê strings básicas ou literais de uma linha, com comentário opcional.
func tomlUnquote(s string) (string, bool) {
	if len(s) < 2 {
		return "", false
	}
	if s[0] == '\'' {
		if end := strings.IndexByte(s[1:], '\''); end >= 0 {
			return s[1 : end+1], true
		}
		return "", false
	}
	if s[0] != '"' {
		return "", false
	}
	escaped := false
	for i := 1; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		if s[i] == '"' {
			v, err := strconv.Unquote(s[:i+1])
			return v, err == nil
		}
	}
	return "", false
}
