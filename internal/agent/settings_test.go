package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const liveSettings = `{
  "unknown": {"deep": [1, 2, {"x": true}]},
  "env": {"ANTHROPIC_MODEL": "opus"},
  "permissions": {"allow": ["Bash"]}
}
`

func TestSettingsRoundTripPreservesUnknownKeysAndPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(liveSettings), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := readSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.set("env", map[string]string{"ANTHROPIC_BASE_URL": "https://example"}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0600", got)
	}

	var got map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("written file is not valid JSON: %v\n%s", err, data)
	}
	if _, ok := got["unknown"]; !ok {
		t.Errorf("unknown key lost: %s", data)
	}
	if string(got["permissions"]) == "" {
		t.Errorf("permissions lost: %s", data)
	}
	if !strings.Contains(string(got["env"]), "example") {
		t.Errorf("env not replaced: %s", got["env"])
	}
	// the edited key keeps its original position, not the end
	if i, j := strings.Index(string(data), `"env"`), strings.Index(string(data), `"permissions"`); i > j {
		t.Errorf("key order changed:\n%s", data)
	}

	// backup of the previous content, with the original mode
	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v; want 1 file", entries, err)
	}
	bpath := filepath.Join(dir, "backups", entries[0].Name())
	bdata, err := os.ReadFile(bpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(bdata) != liveSettings {
		t.Errorf("backup lacks the original content:\n%s", bdata)
	}
	binfo, err := os.Stat(bpath)
	if err != nil {
		t.Fatal(err)
	}
	if got := binfo.Mode().Perm(); got != 0o600 {
		t.Errorf("backup mode = %v, want 0600", got)
	}
}

func TestSettingsGetSetDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	s, err := readSettings(path) // missing: empty document, no error
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if ok, err := s.get("env", &env); ok || err != nil {
		t.Fatalf("get on a missing file = %v, %v; want false, nil", ok, err)
	}
	if err := s.set("env", map[string]string{"A": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(""); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("new file = %v, %v; want 0600", info, err)
	}

	s, err = readSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.get("env", &env); !ok || err != nil || env["A"] != "1" {
		t.Fatalf("get = %v, %v, %v", ok, err, env)
	}
	if err := s.set("env", nil); err != nil { // nil removes
		t.Fatal(err)
	}
	if err := s.save(""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "env") {
		t.Errorf("env should have been removed: %s", data)
	}
}

func TestSettingsRejectsNonObject(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"lista.json": "[1,2]", "quebrado.json": "{\"a\":"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readSettings(path); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
