package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ConfigRecoveryHost approves exact destinations resolved by the adapter,
// including config-directory overrides. Backups cannot choose other paths.
type ConfigRecoveryHost interface {
	RecoveryFiles() []RecoveryFile
}

type RecoveryFile struct {
	Path string
	// Changes validates structure and returns a value-free before/after diff.
	// Unknown values, hook commands, comments and credentials stay hidden.
	Changes func(current, backup []byte) ([]string, error)
}

func (c *Claude) RecoveryFiles() []RecoveryFile {
	return []RecoveryFile{{c.ProviderFile(), recoveryJSONChanges}}
}

func (c *Codex) RecoveryFiles() []RecoveryFile {
	return []RecoveryFile{{c.ProviderFile(), recoveryCodexChanges}, {c.HooksFile(), recoveryJSONChanges}}
}

func (p *Pi) RecoveryFiles() []RecoveryFile {
	return []RecoveryFile{{p.ProviderFile(), recoveryJSONChanges}, {p.settingsFile(), recoveryJSONChanges}}
}

func (c *Crush) RecoveryFiles() []RecoveryFile {
	return []RecoveryFile{{c.ProviderFile(), recoveryCrushChanges}}
}

func recoveryJSONFields(data []byte) (map[string]string, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&root); !json.Valid(data) || err != nil || root == nil {
		return nil, fmt.Errorf("configuration must be a valid JSON object")
	}
	fields := map[string]string{}
	var walk func(string, any)
	walk = func(path string, value any) {
		switch v := value.(type) {
		case map[string]any:
			if len(v) > 0 {
				for k, x := range v {
					walk(path+"/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(k), x)
				}
				return
			}
		case []any:
			if len(v) > 0 {
				for i, x := range v {
					walk(fmt.Sprintf("%s/%d", path, i), x)
				}
				return
			}
		}
		encoded, _ := json.Marshal(value)
		fields[path] = string(encoded)
	}
	for k, v := range root {
		walk("/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(k), v)
	}
	return fields, nil
}

func recoveryJSONChanges(current, backup []byte) ([]string, error) {
	after, err := recoveryJSONFields(backup)
	if err != nil {
		return nil, err
	}
	before, err := recoveryJSONFields(current)
	if err != nil {
		return append([]string{"Current JSON is invalid; the backup is a valid object. All line contents are hidden."}, recoveryLineChanges(current, backup)...), nil
	}
	keys := make([]string, 0, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
	}
	for key := range after {
		if _, ok := before[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	var changes []string
	for _, key := range keys {
		a, had := before[key]
		b, has := after[key]
		label := strconv.Quote(key)
		switch {
		case !had:
			changes = append(changes, "+ "+label+": absent -> [hidden]")
		case !has:
			changes = append(changes, "- "+label+": [hidden] -> absent")
		case a != b:
			changes = append(changes, "~ "+label+": [hidden] -> [hidden] (changed)")
		}
	}
	return changes, nil
}

func recoveryCodexChanges(current, backup []byte) ([]string, error) {
	for _, data := range [][]byte{current, backup} {
		// The surgical scanner cannot prove arbitrary consent keys safe.
		// Fail closed for hook text and Unicode-escaped spellings.
		if !bytes.Equal(current, backup) && (bytes.Contains(data, []byte("hooks")) || bytes.Contains(data, []byte("trusted_hash")) || bytes.Contains(data, []byte(`\u`)) || bytes.Contains(data, []byte(`\U`))) {
			return nil, fmt.Errorf("Codex hook-related text or escaped keys require native review; restore this file in Codex itself")
		}
		lines := splitLines(string(data))
		_, err := tomlValueLines(lines)
		if err != nil {
			return nil, err
		}
	}
	return recoveryLineChanges(current, backup), nil
}

func recoveryCrushChanges(current, backup []byte) ([]string, error) {
	for _, data := range [][]byte{current, backup} {
		for _, kind := range []string{"provider", "hooks"} {
			if _, _, err := crushBlock(splitLines(string(data)), kind); err != nil {
				return nil, err
			}
		}
	}
	return recoveryLineChanges(current, backup), nil
}

var recoveryKey = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*=`)

func recoveryLineChanges(current, backup []byte) []string {
	before, after := splitLines(string(current)), splitLines(string(backup))
	var changes []string
	for i := range max(len(before), len(after)) {
		a, b := "", ""
		if i < len(before) {
			a = before[i]
		}
		if i < len(after) {
			b = after[i]
		}
		if a == b {
			continue
		}
		label := fmt.Sprintf("line %d", i+1)
		if key := recoveryKey.FindStringSubmatch(a); len(key) > 1 {
			label += " (" + key[1] + ")"
		}
		changes = append(changes, "~ "+label+": [hidden] -> [hidden] (changed)")
	}
	return changes
}
