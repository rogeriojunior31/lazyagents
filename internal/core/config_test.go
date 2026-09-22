package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRoundTripKeepsCommentsAndSections(t *testing.T) {
	path := PathsIn(t.TempDir()).ConfigPath()
	writeFile(t, path, "# cabeçalho\ntheme: garoa # linha\nusage:\n  days: 7\n")

	cfg, err := ReadConfig(path)
	if err != nil || cfg.Theme != "garoa" {
		t.Fatalf("ReadConfig: %+v %v", cfg, err)
	}
	cfg.LibraryDir = "~/skills"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{"# cabeçalho", "theme: garoa # linha", "usage:", "days: 7", "libraryDir: ~/skills"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("faltou %q no yaml:\n%s", want, data)
		}
	}
	cfg, err = ReadConfig(path)
	if err != nil || cfg.Theme != "garoa" || cfg.LibraryDir != "~/skills" {
		t.Fatalf("config perdida no round-trip: %+v %v", cfg, err)
	}
	var usage struct{ Days int }
	if err := cfg.Section("usage", &usage); err != nil || usage.Days != 7 {
		t.Errorf("Section(usage) = %+v %v", usage, err)
	}
	usage.Days = 3
	if err := cfg.Section("missing", &usage); err != nil || usage.Days != 3 {
		t.Errorf("Section ausente deve deixar out intacto: %+v %v", usage, err)
	}
	// campo vazio remove a chave
	cfg.Theme = ""
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "theme") || !strings.Contains(string(data), "usage:") {
		t.Errorf("remoção de theme quebrou o yaml:\n%s", data)
	}
}

func TestConfigAbsentAndInvalid(t *testing.T) {
	p := PathsIn(t.TempDir())
	cfg, err := ReadConfig(p.ConfigPath())
	if err != nil || cfg.Theme != "" {
		t.Fatalf("config ausente deve usar o padrão: %+v %v", cfg, err)
	}
	cfg.Theme = "noite"
	if err := cfg.Save(p.ConfigPath()); err != nil {
		t.Fatal(err)
	}
	if cfg, err = ReadConfig(p.ConfigPath()); err != nil || cfg.Theme != "noite" {
		t.Fatalf("Save a partir do zero: %+v %v", cfg, err)
	}
	writeFile(t, p.ConfigPath(), "- lista\n")
	if _, err := ReadConfig(p.ConfigPath()); err == nil {
		t.Error("raiz lista deveria ser erro")
	}
	writeFile(t, p.ConfigPath(), "theme: [\n")
	if _, err := ReadConfig(p.ConfigPath()); err == nil {
		t.Error("yaml inválido deveria ser erro")
	}
}

func TestMigrateConfig(t *testing.T) {
	p := PathsIn(t.TempDir())
	writeFile(t, p.LegacyConfigPath(), `{"theme":"garoa","custom":{"x":1}}`)

	ok, err := MigrateConfig(p)
	if err != nil || !ok {
		t.Fatalf("MigrateConfig = %v %v", ok, err)
	}
	if _, err := os.Stat(p.LegacyConfigPath() + ".migrated"); err != nil {
		t.Error("config.json.migrated não existe")
	}
	if _, err := os.Stat(p.LegacyConfigPath()); !os.IsNotExist(err) {
		t.Error("config.json deveria ter sido renomeado")
	}
	cfg, err := ReadConfig(p.ConfigPath())
	if err != nil || cfg.Theme != "garoa" {
		t.Fatalf("yaml migrado: %+v %v", cfg, err)
	}
	var custom struct{ X int }
	if err := cfg.Section("custom", &custom); err != nil || custom.X != 1 {
		t.Errorf("chave desconhecida perdida na migração: %+v %v", custom, err)
	}
	if ok, err := MigrateConfig(p); ok || err != nil {
		t.Errorf("segunda migração deve ser no-op: %v %v", ok, err)
	}
}

func TestMigrateConfig_InvalidJSONLeavesFilesAlone(t *testing.T) {
	p := PathsIn(t.TempDir())
	writeFile(t, p.LegacyConfigPath(), `{"theme":`)
	if ok, err := MigrateConfig(p); ok || err == nil {
		t.Fatalf("json inválido deveria falhar: %v %v", ok, err)
	}
	if _, err := os.Stat(p.ConfigPath()); !os.IsNotExist(err) {
		t.Error("yaml não deveria ter sido escrito")
	}
	if _, err := os.Stat(p.LegacyConfigPath()); err != nil {
		t.Error("config.json deveria continuar intacto")
	}
	if ok, err := MigrateConfig(PathsIn(t.TempDir())); ok || err != nil {
		t.Errorf("sem json deve ser no-op: %v %v", ok, err)
	}
}
