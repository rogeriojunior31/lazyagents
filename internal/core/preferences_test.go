package core

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

func TestConfigPreferenceSaveGuardsConcurrentWriters(t *testing.T) {
	path := PathsIn(t.TempDir()).ConfigPath()
	writeFile(t, path, "theme: sp-night\nlibraryDir: {future: keep}\nother: stable\n")
	before, _ := os.ReadFile(path)
	cfg, err := ReadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetSectionValue("tui", "startTab", "skills"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "theme: sp-night\nother: written concurrently\n")
	if err := cfg.Save(path, before); !errors.Is(err, fsutil.ErrChanged) {
		t.Fatalf("unguarded config replacement: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "written concurrently") || strings.Contains(string(after), "startTab") {
		t.Fatal("stale save overwrote the new file")
	}
	missing := PathsIn(t.TempDir()).ConfigPath()
	cfg, _ = ReadConfig(missing)
	cfg.Theme = "sp-night"
	writeFile(t, missing, "other: created concurrently\n")
	if err := cfg.Save(missing, nil); !errors.Is(err, fsutil.ErrChanged) {
		t.Fatalf("missing-file guard ignored a new file: %v", err)
	}
}
