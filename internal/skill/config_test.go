package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestLoadPaths_NoConfig(t *testing.T) {
	p := testPaths(t)
	// sem config.json → LibraryDir() = default
	if p.LibraryOverride != "" {
		t.Errorf("LibraryOverride deve ser vazio sem config, got %q", p.LibraryOverride)
	}
	want := filepath.Join(p.DataDir, "skills")
	if got := p.LibraryDir(); got != want {
		t.Errorf("LibraryDir() = %q, want %q", got, want)
	}
}

func TestLoadPaths_WithConfig(t *testing.T) {
	p := testPaths(t)
	customLib := filepath.Join(t.TempDir(), "custom-skills")
	// salva config
	cfgPath := p.ConfigPath()
	if err := (core.Config{LibraryDir: customLib}).Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	// relê
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
	if cfg.Theme != "garoa" || cfg.LibraryDir != newLib {
		t.Errorf("migrate-library apagou o tema: %+v", cfg)
	}
}

func TestMigrateLibrary_MovesSkills(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-mig", validMD("sk-mig", "desc"))

	newLib := filepath.Join(t.TempDir(), "new-lib")
	agents := []agent.Agent{} // sem agentes ativos

	if err := svc.MigrateLibrary(newLib, agents); err != nil {
		t.Fatalf("MigrateLibrary: %v", err)
	}

	// skill deve estar no novo dir
	if _, err := os.Stat(filepath.Join(newLib, "sk-mig", "SKILL.md")); err != nil {
		t.Errorf("skill não encontrada no novo dir: %v", err)
	}

	// skill deve ter sido removida do dir antigo
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk-mig")); !os.IsNotExist(err) {
		t.Error("skill ainda existe no dir antigo após migração")
	}

	// LibraryDir() deve retornar o novo dir
	if got := svc.paths.LibraryDir(); got != newLib {
		t.Errorf("LibraryDir() após migração = %q, want %q", got, newLib)
	}
}

func TestMigrateLibrary_Idempotent(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-idem", validMD("sk-idem", "desc"))

	newLib := filepath.Join(t.TempDir(), "new-lib")
	if err := svc.MigrateLibrary(newLib, nil); err != nil {
		t.Fatalf("1a migração: %v", err)
	}
	if err := svc.MigrateLibrary(newLib, nil); err != nil {
		t.Fatalf("2a migração (idempotente): %v", err)
	}
	// skill ainda no novo dir após segunda chamada
	if _, err := os.Stat(filepath.Join(newLib, "sk-idem", "SKILL.md")); err != nil {
		t.Errorf("skill desapareceu após 2ª migração: %v", err)
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

	// ativa a skill no agente (cria symlink)
	skills, _ := svc.Scan([]agent.Agent{ag})
	var sk Skill
	for _, s := range skills {
		if s.Dir == "sk-sym" {
			sk = s
		}
	}
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	newLib := filepath.Join(t.TempDir(), "new-lib")
	if err := svc.MigrateLibrary(newLib, []agent.Agent{ag}); err != nil {
		t.Fatalf("MigrateLibrary: %v", err)
	}

	// symlink deve apontar para o novo dir
	linkPath := filepath.Join(agentDir, "sk-sym")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("lendo symlink após migração: %v", err)
	}
	wantTarget := filepath.Join(newLib, "sk-sym")
	if target != wantTarget {
		t.Errorf("symlink aponta para %q, want %q", target, wantTarget)
	}
}

func TestScan_LibraryDirAsAgentReadDir_NotLocal(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-shared", validMD("sk-shared", "desc"))

	// agente com ReadDir == LibraryDir (simulando ~/.agents/skills com lib override)
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
				t.Error("skill da biblioteca marcada como Local — não deveria")
			}
			if !st.On {
				t.Error("skill da biblioteca deveria estar On")
			}
		}
	}
	if !found {
		t.Error("sk-shared não encontrada no scan")
	}
}
