package core

import (
	"testing"
)

func TestThemeRoundTripPreservesConfig(t *testing.T) {
	path := PathsIn(t.TempDir()).ConfigPath()
	raw, cfg, err := ReadConfigRaw(path)
	if err != nil || cfg.Theme != "" {
		t.Fatalf("config ausente deve usar o padrão: %q %v", cfg.Theme, err)
	}
	raw["custom"] = []byte(`{"enabled":true}`)
	if err := SaveConfig(path, raw, Config{LibraryDir: "~/skills", Theme: "garoa"}); err != nil {
		t.Fatal(err)
	}
	raw, cfg, err = ReadConfigRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "garoa" || cfg.LibraryDir != "~/skills" || string(raw["custom"]) == "" {
		t.Fatalf("config perdida no round-trip: %+v %s", cfg, raw["custom"])
	}
}
