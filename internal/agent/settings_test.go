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
	if err := s.set("env", map[string]string{"ANTHROPIC_BASE_URL": "https://exemplo"}); err != nil {
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
		t.Errorf("permissão = %v, queria 0600", got)
	}

	var got map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("arquivo gravado não é JSON válido: %v\n%s", err, data)
	}
	if _, ok := got["unknown"]; !ok {
		t.Errorf("chave desconhecida sumiu: %s", data)
	}
	if string(got["permissions"]) == "" {
		t.Errorf("permissions sumiu: %s", data)
	}
	if !strings.Contains(string(got["env"]), "exemplo") {
		t.Errorf("env não foi trocado: %s", got["env"])
	}
	// A chave editada continua na posição original, não no fim.
	if i, j := strings.Index(string(data), `"env"`), strings.Index(string(data), `"permissions"`); i > j {
		t.Errorf("ordem das chaves mudou:\n%s", data)
	}

	// Backup do conteúdo anterior, com a mesma permissão do original.
	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v; queria 1 arquivo", entries, err)
	}
	bpath := filepath.Join(dir, "backups", entries[0].Name())
	bdata, err := os.ReadFile(bpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(bdata) != liveSettings {
		t.Errorf("backup não tem o conteúdo original:\n%s", bdata)
	}
	binfo, err := os.Stat(bpath)
	if err != nil {
		t.Fatal(err)
	}
	if got := binfo.Mode().Perm(); got != 0o600 {
		t.Errorf("permissão do backup = %v, queria 0600", got)
	}
}

func TestSettingsGetSetDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	s, err := readSettings(path) // ausente: documento vazio, sem erro
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if ok, err := s.get("env", &env); ok || err != nil {
		t.Fatalf("get em arquivo ausente = %v, %v; queria false, nil", ok, err)
	}
	if err := s.set("env", map[string]string{"A": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(""); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("arquivo novo = %v, %v; queria 0600", info, err)
	}

	s, err = readSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.get("env", &env); !ok || err != nil || env["A"] != "1" {
		t.Fatalf("get = %v, %v, %v", ok, err, env)
	}
	if err := s.set("env", nil); err != nil { // nil remove
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
		t.Errorf("env deveria ter sido removida: %s", data)
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
			t.Errorf("%s: queria erro", name)
		}
	}
}
