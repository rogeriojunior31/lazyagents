package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// migAdapter é um agent.Adapter mínimo cujo Detect() devolve um Agent com o
// ManagedDir dado — só o suficiente para exercitar o re-apontamento de symlinks.
type migAdapter struct{ managedDir string }

func (a migAdapter) ID() string { return "mig" }
func (a migAdapter) Detect() agent.Agent {
	return agent.Agent{ID: "mig", Installed: true, ManagedDir: a.managedDir}
}
func (a migAdapter) ListSessions() ([]agent.Session, error)           { return nil, nil }
func (a migAdapter) ResumeCmd(agent.Session) ([]string, string, bool) { return nil, "", false }
func (a migAdapter) Transcript(agent.Session) ([]agent.Entry, error)  { return nil, nil }
func (a migAdapter) DeleteSession(agent.Session, string) error        { return nil }

// legacyLayout monta um ~/.lazyskills fake com uma skill, backups, profiles e
// config, e devolve o path legado.
func legacyLayout(t *testing.T, p Paths) string {
	t.Helper()
	legacy := filepath.Join(p.Home, ".lazyskills")
	writeSkill(t, filepath.Join(legacy, "skills"), "sk-old", validMD("sk-old", "desc"))
	if err := os.MkdirAll(filepath.Join(legacy, "backups"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "backups", "b.tar.gz"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "profiles.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return legacy
}

func TestEnsureMigrated_MovesEverything(t *testing.T) {
	p := testPaths(t)
	legacy := legacyLayout(t, p)

	// agente com a skill ativada (symlink apontando p/ a lib legada)
	agentDir := filepath.Join(t.TempDir(), "agent-skills")
	mustSymlink(t, filepath.Join(legacy, "skills", "sk-old"), filepath.Join(agentDir, "sk-old"))

	migrated, err := EnsureMigrated(p, []agent.Adapter{migAdapter{managedDir: agentDir}})
	if err != nil {
		t.Fatalf("EnsureMigrated: %v", err)
	}
	if !migrated {
		t.Error("migrated = false, esperava true")
	}

	// arquivos nos novos locais XDG
	checks := map[string]string{
		"skill":    filepath.Join(p.LibraryDir(), "sk-old", "SKILL.md"),
		"config":   p.ConfigPath(),
		"profiles": p.ProfilesPath(),
		"backup":   filepath.Join(p.BackupsDir(), "b.tar.gz"),
	}
	for label, path := range checks {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s não encontrado no novo local (%s): %v", label, path, err)
		}
	}

	// symlink do agente re-apontado para o novo LibraryDir
	target, err := os.Readlink(filepath.Join(agentDir, "sk-old"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if want := filepath.Join(p.LibraryDir(), "sk-old"); target != want {
		t.Errorf("symlink aponta p/ %q, want %q", target, want)
	}

	// legado removido (ficou vazio)
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("~/.lazyskills ainda existe após migração")
	}
}

func TestEnsureMigrated_NoLegacy(t *testing.T) {
	p := testPaths(t)
	migrated, err := EnsureMigrated(p, nil)
	if err != nil {
		t.Fatalf("EnsureMigrated: %v", err)
	}
	if migrated {
		t.Error("migrated = true sem ~/.lazyskills")
	}
}

func TestEnsureMigrated_Idempotent(t *testing.T) {
	p := testPaths(t)
	legacyLayout(t, p)

	if _, err := EnsureMigrated(p, nil); err != nil {
		t.Fatalf("1ª migração: %v", err)
	}
	migrated, err := EnsureMigrated(p, nil)
	if err != nil {
		t.Fatalf("2ª migração: %v", err)
	}
	if migrated {
		t.Error("2ª migração reportou migrated = true (deveria ser no-op)")
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk-old", "SKILL.md")); err != nil {
		t.Errorf("skill sumiu após 2ª migração: %v", err)
	}
}

func TestEnsureMigrated_CustomLibraryDirNotMoved(t *testing.T) {
	p := testPaths(t)
	legacy := legacyLayout(t, p)
	// config legado com libraryDir custom → skills NÃO devem ser movidas
	custom := filepath.Join(t.TempDir(), "custom-lib")
	cfg, _ := json.Marshal(map[string]string{"libraryDir": custom})
	if err := os.WriteFile(filepath.Join(legacy, "config.json"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureMigrated(p, nil); err != nil {
		t.Fatalf("EnsureMigrated: %v", err)
	}

	// skills continuam no dir legado (não movidas)
	if _, err := os.Stat(filepath.Join(legacy, "skills", "sk-old")); err != nil {
		t.Errorf("skills com libraryDir custom foram movidas indevidamente: %v", err)
	}
	// mas config foi migrada
	if _, err := os.Stat(p.ConfigPath()); err != nil {
		t.Errorf("config.json não migrado: %v", err)
	}
}

func TestEnsureMigrated_RenamedXDG(t *testing.T) {
	p := testPaths(t)
	oldData := filepath.Join(filepath.Dir(p.DataDir), "lazyskills")
	oldCfg := filepath.Join(filepath.Dir(p.ConfigDir), "lazyskills")
	oldLib := filepath.Join(oldData, "skills")
	writeSkill(t, oldLib, "sk-ren", validMD("sk-ren", "desc"))
	if err := os.MkdirAll(oldCfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldCfg, "config.json"), []byte(`{"x":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	agentDir := filepath.Join(t.TempDir(), "agent-skills")
	mustSymlink(t, filepath.Join(oldLib, "sk-ren"), filepath.Join(agentDir, "sk-ren"))

	migrated, err := EnsureMigrated(p, []agent.Adapter{migAdapter{managedDir: agentDir}})
	if err != nil {
		t.Fatalf("EnsureMigrated: %v", err)
	}
	if !migrated {
		t.Error("migrated = false, esperava true")
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk-ren", "SKILL.md")); err != nil {
		t.Errorf("skill não migrou: %v", err)
	}
	if _, err := os.Stat(p.ConfigPath()); err != nil {
		t.Errorf("config não migrou: %v", err)
	}
	for _, old := range []string{oldData, oldCfg} {
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Errorf("%s ainda existe", old)
		}
	}
	target, err := os.Readlink(filepath.Join(agentDir, "sk-ren"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if want := filepath.Join(p.LibraryDir(), "sk-ren"); target != want {
		t.Errorf("symlink aponta p/ %q, want %q", target, want)
	}

	// idempotente
	again, err := EnsureMigrated(p, nil)
	if err != nil || again {
		t.Errorf("segunda execução: migrated=%v err=%v", again, err)
	}
}
