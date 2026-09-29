package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestLoadPaths_NoConfig(t *testing.T) {
	p := testPaths(t)
	// no config → LibraryDir() = default
	if p.LibraryOverride != "" {
		t.Errorf("LibraryOverride should be empty without config, got %q", p.LibraryOverride)
	}
	want := filepath.Join(p.DataDir, "skills")
	if got := p.LibraryDir(); got != want {
		t.Errorf("LibraryDir() = %q, want %q", got, want)
	}
}

func TestLoadPaths_WithConfig(t *testing.T) {
	p := testPaths(t)
	customLib := filepath.Join(t.TempDir(), "custom-skills")
	// save config
	cfgPath := p.ConfigPath()
	if err := (core.Config{LibraryDir: customLib}).Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	// read it back
	cfg, err := core.ReadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LibraryDir != customLib {
		t.Errorf("config.LibraryDir = %q, want %q", cfg.LibraryDir, customLib)
	}
}

func TestMigrateLibrary_KeepsTheme(t *testing.T) {
	p := testPaths(t)
	if err := (core.Config{Theme: "garoa"}).Save(p.ConfigPath()); err != nil {
		t.Fatal(err)
	}
	newLib := filepath.Join(t.TempDir(), "new-lib")
	if err := New(p).MigrateLibrary(newLib, nil); err != nil {
		t.Fatalf("MigrateLibrary: %v", err)
	}
	cfg, err := core.ReadConfig(p.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "garoa" || cfg.LibraryDir != physical(t, newLib) {
		t.Errorf("migrate-library dropped the theme: %+v", cfg)
	}
}

func TestMigrateLibrary_MovesSkills(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-mig", validMD("sk-mig", "desc"))

	newLib := filepath.Join(t.TempDir(), "new-lib")
	agents := []agent.Agent{} // no active agents

	if err := svc.MigrateLibrary(newLib, agents); err != nil {
		t.Fatalf("MigrateLibrary: %v", err)
	}

	if _, err := os.Stat(filepath.Join(newLib, "sk-mig", "SKILL.md")); err != nil {
		t.Errorf("skill not found in the new dir: %v", err)
	}

	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk-mig")); !os.IsNotExist(err) {
		t.Error("skill still in the old dir after migration")
	}

	if got, want := svc.paths.LibraryDir(), physical(t, newLib); got != want {
		t.Errorf("LibraryDir() after migration = %q, want %q", got, want)
	}
}

// physical resolves symlinks and short names the way MigrateLibrary does: the
// temp dir is /var → /private/var on macOS and RUNNER~1 on Windows runners.
func physical(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestMigrateLibrary_Idempotent(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-idem", validMD("sk-idem", "desc"))

	newLib := filepath.Join(t.TempDir(), "new-lib")
	if err := svc.MigrateLibrary(newLib, nil); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := svc.MigrateLibrary(newLib, nil); err != nil {
		t.Fatalf("second migration (idempotent): %v", err)
	}
	if _, err := os.Stat(filepath.Join(newLib, "sk-idem", "SKILL.md")); err != nil {
		t.Errorf("skill gone after the second migration: %v", err)
	}
}

func TestMigrateLibrary_UpdatesSymlinks(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	agentDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ag := agent.Agent{
		ID: "test-ag", Name: "test-ag", Short: "T",
		Installed: true, ManagedDir: agentDir, ReadDirs: []string{agentDir},
	}

	writeSkill(t, p.LibraryDir(), "sk-sym", validMD("sk-sym", "desc"))

	// enable the skill in the agent (creates the symlink)
	skills, _ := svc.Scan([]agent.Agent{ag})
	var sk Skill
	for _, s := range skills {
		if s.Dir == "sk-sym" {
			sk = s
		}
	}
	if err := svc.Enable(sk, ag, nil); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	newLib := filepath.Join(t.TempDir(), "new-lib")
	if err := svc.MigrateLibrary(newLib, []agent.Agent{ag}); err != nil {
		t.Fatalf("MigrateLibrary: %v", err)
	}

	linkPath := filepath.Join(agentDir, "sk-sym")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("reading symlink after migration: %v", err)
	}
	wantTarget := filepath.Join(physical(t, newLib), "sk-sym")
	if target != wantTarget {
		t.Errorf("symlink points to %q, want %q", target, wantTarget)
	}
}

func TestScan_LibraryDirAsAgentReadDir_NotLocal(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-shared", validMD("sk-shared", "desc"))

	// agent with ReadDir == LibraryDir (like ~/.agents/skills with a library override)
	ag := agent.Agent{
		ID:         "test-shared",
		Name:       "Test",
		Short:      "T",
		Installed:  true,
		ManagedDir: p.LibraryDir(), // managed = library dir
		ReadDirs:   []string{p.LibraryDir()},
	}

	skills, err := svc.Scan([]agent.Agent{ag})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range skills {
		if s.Dir == "sk-shared" {
			found = true
			st := s.States["test-shared"]
			if st.Local {
				t.Error("library skill marked Local")
			}
			if !st.On {
				t.Error("library skill should be On")
			}
		}
	}
	if !found {
		t.Error("sk-shared not found by the scan")
	}
}
