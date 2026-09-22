package core

import (
	"path/filepath"
	"testing"
)

func TestPathsIn(t *testing.T) {
	home := t.TempDir()
	p := PathsIn(home)
	if want := filepath.Join(home, ".local", "share", "lazyagents", "skills"); p.LibraryDir() != want {
		t.Errorf("LibraryDir = %q, want %q", p.LibraryDir(), want)
	}
	if want := filepath.Join(home, ".local", "share", "lazyagents", "exports"); p.ExportsDir() != want {
		t.Errorf("ExportsDir = %q, want %q", p.ExportsDir(), want)
	}
}

func TestExpandHomeAndTilde(t *testing.T) {
	p := PathsIn("/h")
	cases := map[string]string{"~": "/h", "~/x/y": "/h/x/y", "/abs": "/abs", "rel": "rel"}
	for in, want := range cases {
		if got := p.ExpandHome(in); got != want {
			t.Errorf("ExpandHome(%q) = %q, want %q", in, got, want)
		}
	}
	if got := p.Tilde("/h/proj"); got != "~/proj" {
		t.Errorf("Tilde = %q", got)
	}
}

func TestWithConfig_Override(t *testing.T) {
	p := PathsIn(t.TempDir())
	raw, _, _ := ReadConfigRaw(p.ConfigPath())
	if err := SaveConfig(p.ConfigPath(), raw, Config{LibraryDir: "~/.agents/skills"}); err != nil {
		t.Fatal(err)
	}
	got := p.WithConfig().LibraryDir()
	if want := filepath.Join(p.Home, ".agents", "skills"); got != want {
		t.Errorf("LibraryDir = %q, want %q", got, want)
	}
}
