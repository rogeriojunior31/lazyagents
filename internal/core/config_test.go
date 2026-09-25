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
	writeFile(t, path, "# header\ntheme: garoa # line\nusage:\n  days: 7\n")

	cfg, err := ReadConfig(path)
	if err != nil || cfg.Theme != "garoa" {
		t.Fatalf("ReadConfig: %+v %v", cfg, err)
	}
	cfg.LibraryDir = "~/skills"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{"# header", "theme: garoa # line", "usage:", "days: 7", "libraryDir: ~/skills"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q in yaml:\n%s", want, data)
		}
	}
	cfg, err = ReadConfig(path)
	if err != nil || cfg.Theme != "garoa" || cfg.LibraryDir != "~/skills" {
		t.Fatalf("config lost in round-trip: %+v %v", cfg, err)
	}
	var usage struct{ Days int }
	if err := cfg.Section("usage", &usage); err != nil || usage.Days != 7 {
		t.Errorf("Section(usage) = %+v %v", usage, err)
	}
	usage.Days = 3
	if err := cfg.Section("missing", &usage); err != nil || usage.Days != 3 {
		t.Errorf("missing Section must leave out as is: %+v %v", usage, err)
	}
	// an empty field removes the key
	cfg.Theme = ""
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "theme") || !strings.Contains(string(data), "usage:") {
		t.Errorf("removing theme broke the yaml:\n%s", data)
	}
}

func TestConfigAbsentAndInvalid(t *testing.T) {
	p := PathsIn(t.TempDir())
	cfg, err := ReadConfig(p.ConfigPath())
	if err != nil || cfg.Theme != "" {
		t.Fatalf("missing config must use the default: %+v %v", cfg, err)
	}
	cfg.Theme = "sp-night"
	if err := cfg.Save(p.ConfigPath()); err != nil {
		t.Fatal(err)
	}
	if cfg, err = ReadConfig(p.ConfigPath()); err != nil || cfg.Theme != "sp-night" {
		t.Fatalf("Save from scratch: %+v %v", cfg, err)
	}
	writeFile(t, p.ConfigPath(), "- list\n")
	if _, err := ReadConfig(p.ConfigPath()); err == nil {
		t.Error("a list root should be an error")
	}
	writeFile(t, p.ConfigPath(), "theme: [\n")
	if _, err := ReadConfig(p.ConfigPath()); err == nil {
		t.Error("invalid yaml should be an error")
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
		t.Error("config.json.migrated does not exist")
	}
	if _, err := os.Stat(p.LegacyConfigPath()); !os.IsNotExist(err) {
		t.Error("config.json should have been renamed")
	}
	cfg, err := ReadConfig(p.ConfigPath())
	if err != nil || cfg.Theme != "garoa" {
		t.Fatalf("migrated yaml: %+v %v", cfg, err)
	}
	var custom struct{ X int }
	if err := cfg.Section("custom", &custom); err != nil || custom.X != 1 {
		t.Errorf("unknown key lost in migration: %+v %v", custom, err)
	}
	if ok, err := MigrateConfig(p); ok || err != nil {
		t.Errorf("second migration must be a no-op: %v %v", ok, err)
	}
}

func TestMigrateConfig_InvalidJSONLeavesFilesAlone(t *testing.T) {
	p := PathsIn(t.TempDir())
	writeFile(t, p.LegacyConfigPath(), `{"theme":`)
	if ok, err := MigrateConfig(p); ok || err == nil {
		t.Fatalf("invalid json should fail: %v %v", ok, err)
	}
	if _, err := os.Stat(p.ConfigPath()); !os.IsNotExist(err) {
		t.Error("yaml should not have been written")
	}
	if _, err := os.Stat(p.LegacyConfigPath()); err != nil {
		t.Error("config.json should stay intact")
	}
	if ok, err := MigrateConfig(PathsIn(t.TempDir())); ok || err != nil {
		t.Errorf("no json must be a no-op: %v %v", ok, err)
	}
}
