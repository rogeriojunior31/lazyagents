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

// Codex config is TOML that also carries foreign state ([projects.*],
// [hooks.state.*] with trust hashes). Reserializing it would rewrite all that,
// so lazyagents edits only the blocks between these markers and copies every
// other line verbatim.
const (
	codexBlockStart = "# lazyagents — managed block start (do not edit by hand)"
	codexBlockEnd   = "# lazyagents — managed block end"
	// codexProviderID is the one provider table lazyagents manages: profiles live
	// in providers.json, config.toml holds only the active one.
	codexProviderID = "lazyagents"
	// codexPrevModel keeps the user's `model` inside the block: the profile must
	// replace that key (TOML forbids duplicates), so the old value rides along
	// as a comment and comes back on Clear.
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

func (c *Codex) ProviderFile() string { return filepath.Join(c.configDir(), "config.toml") }

func (c *Codex) ReadProvider() (ProviderProfile, bool, error) {
	data, err := os.ReadFile(c.ProviderFile())
	if err != nil {
		if os.IsNotExist(err) {
			return ProviderProfile{}, false, nil
		}
		return ProviderProfile{}, false, fmt.Errorf("reading %s: %w", c.ProviderFile(), err)
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
		return fmt.Errorf("Codex only supports wireApi responses")
	}
	if p.BaseURL == "" {
		return fmt.Errorf("the profile needs baseUrl (it becomes base_url in [model_providers])")
	}
	if p.Token != "" && p.EnvKey == "" {
		// Codex does not read tokens from config.toml, only the env var named by
		// env_key. Failing beats writing a profile that silently has no token.
		return fmt.Errorf("Codex reads the token from an environment variable: set envKey in the profile and export that variable")
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

// writeTOML rewrites config.toml with the managed blocks replaced: the
// top-level keys block goes before the first table (a bare key after a
// [header] would belong to it) and the provider table block goes last.
// Empty blocks are dropped.
func (c *Codex) writeTOML(backupsDir string, top, table []string, setsModel bool) error {
	path := c.ProviderFile()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	all := splitLines(string(data))
	inValue, err := tomlValueLines(all)
	if err != nil {
		return err
	}
	depth := 0
	for i, line := range all {
		if inValue[i] {
			continue // inside a multiline string or array: not a marker
		}
		switch start, end := codexMarker(strings.TrimSpace(line)); {
		case start:
			if depth != 0 {
				return fmt.Errorf("nested lazyagents blocks; config left untouched")
			}
			depth++
		case end:
			if depth != 1 {
				return fmt.Errorf("lazyagents block end without a start; config left untouched")
			}
			depth--
		}
	}
	if depth != 0 {
		return fmt.Errorf("lazyagents block without an end; config left untouched")
	}
	prevModel := codexPrevValueOf(all, codexPrevModel, codexLegacyPrevModel)
	prevProvider := codexPrevValueOf(all, codexPrevProvider, codexLegacyPrevProvider)
	lines := stripCodexBlocks(all)
	if len(top) > 0 {
		_, providers := parseCodexTOML(strings.Join(lines, "\n"))
		if _, exists := providers[codexProviderID]; exists {
			return fmt.Errorf("table model_providers.lazyagents already exists outside the managed block")
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
		// Clear: restore the user's model as a top-level key (its exact original
		// line is unknown, and the top is where top-level keys are valid).
		lines = append([]string{fmt.Sprintf("model = %s", tomlString(prevModel))}, lines...)
	}

	if len(top) > 0 {
		at := len(lines)
		inValue, err := tomlValueLines(lines)
		if err != nil {
			return err
		}
		for i, l := range lines {
			if !inValue[i] && strings.HasPrefix(strings.TrimSpace(l), "[") {
				at = i // the first [table] header
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
		return fmt.Errorf("writing %s: %w", path, err)
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

// takeTopKey removes a top-level key (outside tables) and returns its value:
// the managed block declares its own, and a duplicate key breaks the TOML.
func takeTopKey(lines []string, wanted string) ([]string, string, error) {
	inValue, err := tomlValueLines(lines)
	if err != nil {
		return nil, "", err
	}
	out := make([]string, 0, len(lines))
	value := ""
	inTable := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if !inValue[i] && strings.HasPrefix(trimmed, "[") {
			inTable = true
		}
		if !inTable && !inValue[i] {
			if key, raw, ok := strings.Cut(trimmed, "="); ok && strings.Trim(strings.TrimSpace(key), `"'`) == wanted {
				if v, ok := tomlUnquote(strings.TrimSpace(raw)); ok {
					value = v
					continue
				}
				return nil, "", fmt.Errorf("unsupported value for %s; config left untouched", wanted)
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

// stripCodexBlocks removes every managed block, markers included (validated
// in writeTOML), plus the blank line wrapCodexBlock wrote after it, so apply
// then clear gives back the exact original file.
func stripCodexBlocks(lines []string) []string {
	inValue, _ := tomlValueLines(lines) // nil on error: callers checked the file first
	out := make([]string, 0, len(lines))
	inBlock, justClosed := false, false
	for i, l := range lines {
		start, end := codexMarker(strings.TrimSpace(l))
		if inValue != nil && inValue[i] {
			start, end = false, false
		}
		switch {
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
	// the blank line that closed the block is not kept at the end
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

// parseCodexTOML extracts only what providers need: top-level keys and
// [model_providers.<id>] tables, string values only. Not a TOML parser: other
// constructs are ignored, which is safe because writing never depends on it.
func parseCodexTOML(data string) (top map[string]string, providers map[string]map[string]string) {
	top = map[string]string{}
	providers = map[string]map[string]string{}
	table := ""
	lines := splitLines(data)
	inValue, err := tomlValueLines(lines)
	if err != nil {
		return top, providers
	}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || inValue[i] {
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

// tomlString writes a TOML basic string.
func tomlString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// tomlUnquote reads single-line basic or literal strings, with an optional comment.
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
