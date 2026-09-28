package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// The registry must build tabs and commands without duplicate names.
func TestFeaturesRegistry(t *testing.T) {
	d, err := LoadWith(core.PathsIn(t.TempDir()), "test")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range d.Modules() {
		if seen["tab:"+m.ID()] {
			t.Errorf("duplicate tab %q", m.ID())
		}
		seen["tab:"+m.ID()] = true
	}
	for _, c := range d.Commands() {
		if seen[c.Name] {
			t.Errorf("duplicate command %q", c.Name)
		}
		seen[c.Name] = true
	}
	for _, want := range []string{"tab:skills", "tab:sessions", "tab:agents", "tab:providers", "tab:usage", "list", "sessions", "provider", "doctor"} {
		if !seen[want] {
			t.Errorf("%q missing from the registry", want)
		}
	}
}

// Booting with a legacy config.json migrates it to yaml and applies the theme.
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
	if d.Deps.Config.Theme != "garoa" || len(d.Deps.Notices()) != 1 {
		t.Errorf("Config = %+v, Notices = %v", d.Deps.Config, d.Deps.Notices())
	}
	if _, err := os.Stat(p.ConfigPath()); err != nil {
		t.Error("config.yaml was not created")
	}
}

// External plugins become tabs and commands; a reserved id is skipped.
func TestPluginsRegistered(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake plugin is a POSIX shell script")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh in PATH")
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
	// Two of the three binaries have reserved ids (tab skills, command list):
	// only hello is left, and the others become boot notices.
	reserved := 0
	for _, n := range d.Deps.Notices() {
		if strings.Contains(n, "id reserved") {
			reserved++
		}
	}
	ids := map[string]bool{}
	for _, m := range d.Modules() {
		ids[m.ID()] = true
	}
	if !ids["hello"] || reserved != 2 {
		t.Fatalf("tabs = %v, reserved-id notices = %d", ids, reserved)
	}
	var out bytes.Buffer
	c := cli.Context{Out: &out, Err: &out, Paths: d.Deps.Paths, Agents: d.Deps.Agents}
	if code := cli.Run([]string{"hello", "x"}, c, d.Commands()); code != 4 {
		t.Errorf("pass-through exit = %d, want 4", code)
	}
	d.Deps.Agents() // the doctor detects them anyway
	if code := cli.Run([]string{"doctor"}, c, d.Commands()); code != 1 || !strings.Contains(out.String(), "✓ hello") || !strings.Contains(out.String(), "id reserved") {
		t.Errorf("doctor exit=%d output:\n%s", code, out.String())
	}
}

// Usage and Agents (read-only tabs) end the bar, in that order, even with
// plugin tabs registered.
func TestInfoTabsAreAlwaysLast(t *testing.T) {
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
	if len(mods) < 2 || mods[len(mods)-2].ID() != "usage" || mods[len(mods)-1].ID() != "agents" {
		var ids []string
		for _, m := range mods {
			ids = append(ids, m.ID())
		}
		t.Fatalf("Usage and Agents should end the bar: %v", ids)
	}
}
