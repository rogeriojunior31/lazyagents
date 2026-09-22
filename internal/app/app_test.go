package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
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

// Plugins externos viram abas e comandos; id reservado é pulado.
func TestPluginsRegistered(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sem sh no PATH")
	}
	p := core.PathsIn(t.TempDir())
	if err := os.MkdirAll(p.PluginsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$1\" in serve) read i; echo '{\"type\":\"manifest\",\"title\":\"Hi\"}'; cat >/dev/null;; *) exit 4;; esac\n"
	for _, name := range []string{"hello", "skills", "list"} {
		if err := os.WriteFile(filepath.Join(p.PluginsDir(), name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	d, err := LoadWith(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	if len(d.plugins) != 1 || d.plugins[0].ID != "hello" || len(d.pluginWarn) != 2 {
		t.Fatalf("plugins = %+v, avisos = %v", d.plugins, d.pluginWarn)
	}
	ids := map[string]bool{}
	for _, m := range d.Modules() {
		ids[m.ID()] = true
	}
	if !ids["hello"] {
		t.Error("aba hello não registrada")
	}
	var out bytes.Buffer
	c := cli.Context{Out: &out, Err: &out, Paths: d.Paths, Agents: d.Agents}
	if code := cli.Run([]string{"hello", "x"}, c, d.Commands()); code != 4 {
		t.Errorf("pass-through exit = %d, want 4", code)
	}
	d.Agents() // já detectado pelo doctor de qualquer forma
	if code := cli.Run([]string{"doctor"}, c, d.Commands()); code != 1 || !strings.Contains(out.String(), "✓ hello") || !strings.Contains(out.String(), "reservado") {
		t.Errorf("doctor exit=%d saída:\n%s", code, out.String())
	}
}

// A aba Uso é sempre a última, mesmo com abas de plugin registradas.
func TestUsageTabIsAlwaysLast(t *testing.T) {
	p := core.PathsIn(t.TempDir())
	if _, err := exec.LookPath("sh"); err == nil {
		if err := os.MkdirAll(p.PluginsDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		script := "#!/bin/sh\ncase \"$1\" in serve) read i; echo '{\"type\":\"manifest\",\"title\":\"Hi\"}'; cat >/dev/null;; *) exit 0;; esac\n"
		if err := os.WriteFile(filepath.Join(p.PluginsDir(), "zz"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	d, err := LoadWith(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	mods := d.Modules()
	if len(mods) == 0 || mods[len(mods)-1].ID() != "usage" {
		var ids []string
		for _, m := range mods {
			ids = append(ids, m.ID())
		}
		t.Fatalf("Uso deveria ser a última aba: %v", ids)
	}
}
