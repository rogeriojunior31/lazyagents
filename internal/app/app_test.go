package app

import (
	"os"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// O registro precisa gerar abas e comandos sem nome duplicado.
func TestFeaturesRegistry(t *testing.T) {
	d, err := LoadWith(core.PathsIn(t.TempDir()), "test")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range d.Modules() {
		if seen["tab:"+m.ID()] {
			t.Errorf("aba duplicada %q", m.ID())
		}
		seen["tab:"+m.ID()] = true
	}
	for _, c := range d.Commands() {
		if seen[c.Name] {
			t.Errorf("comando duplicado %q", c.Name)
		}
		seen[c.Name] = true
	}
	for _, want := range []string{"tab:skills", "tab:sessions", "tab:agents", "list", "sessions", "doctor"} {
		if !seen[want] {
			t.Errorf("faltou %q no registro", want)
		}
	}
}

// Boot com config.json legado migra para yaml e aplica o tema.
func TestLoadWith_MigratesLegacyConfig(t *testing.T) {
	p := core.PathsIn(t.TempDir())
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.LegacyConfigPath(), []byte(`{"theme":"garoa"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := LoadWith(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	if d.Config.Theme != "garoa" || len(d.Notices) != 1 {
		t.Errorf("Config = %+v, Notices = %v", d.Config, d.Notices)
	}
	if _, err := os.Stat(p.ConfigPath()); err != nil {
		t.Error("config.yaml não foi criado")
	}
}
