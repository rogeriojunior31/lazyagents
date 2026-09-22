package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// fakeHost é um agente que suporta provedores, guardando o "arquivo" em
// memória.
type fakeHost struct {
	id        string
	installed bool
	applied   *agent.ProviderProfile
	backups   string
}

func (f *fakeHost) ID() string { return f.id }
func (f *fakeHost) Detect() agent.Agent {
	return agent.Agent{ID: f.id, Name: strings.ToUpper(f.id), Installed: f.installed}
}
func (f *fakeHost) ListSessions() ([]agent.Session, error)           { return nil, nil }
func (f *fakeHost) ResumeCmd(agent.Session) ([]string, string, bool) { return nil, "", false }
func (f *fakeHost) Transcript(agent.Session) ([]agent.Entry, error)  { return nil, nil }
func (f *fakeHost) DeleteSession(agent.Session, string) error        { return nil }
func (f *fakeHost) ProviderFile() string                             { return "/tmp/" + f.id + ".json" }
func (f *fakeHost) ReadProvider() (agent.ProviderProfile, bool, error) {
	if f.applied == nil {
		return agent.ProviderProfile{}, false, nil
	}
	return f.applied.Redacted(), true, nil
}
func (f *fakeHost) ApplyProvider(p agent.ProviderProfile, backupsDir string) error {
	f.applied, f.backups = &p, backupsDir
	return nil
}
func (f *fakeHost) ClearProvider(string) error { f.applied = nil; return nil }

// plainAdapter não suporta provedores: tem que ficar de fora do Status.
type plainAdapter struct{ id string }

func (p *plainAdapter) ID() string { return p.id }
func (p *plainAdapter) Detect() agent.Agent {
	return agent.Agent{ID: p.id, Installed: true}
}
func (p *plainAdapter) ListSessions() ([]agent.Session, error)           { return nil, nil }
func (p *plainAdapter) ResumeCmd(agent.Session) ([]string, string, bool) { return nil, "", false }
func (p *plainAdapter) Transcript(agent.Session) ([]agent.Entry, error)  { return nil, nil }
func (p *plainAdapter) DeleteSession(agent.Session, string) error        { return nil }

func TestProfilesRoundTripAndPermission(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	svc := New(nil, paths)

	if got, err := svc.Profiles(); err != nil || got != nil {
		t.Fatalf("sem arquivo = %v, %v", got, err)
	}
	if err := svc.Save(agent.ProviderProfile{Name: "z-local", BaseURL: "http://localhost"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(agent.ProviderProfile{Name: "a-nuvem", BaseURL: "https://nuvem", Token: "segredo"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(paths.ProvidersPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissão = %v, queria 0600 (pode ter token)", info.Mode().Perm())
	}

	got, err := svc.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a-nuvem" { // ordem alfabética
		t.Fatalf("profiles = %+v", got)
	}
	if got[0].Token != "segredo" {
		t.Errorf("token não sobreviveu: %+v", got[0].Redacted())
	}

	// mesmo nome substitui, não duplica
	if err := svc.Save(agent.ProviderProfile{Name: "z-local", Model: "qwen"}); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Profiles()
	if len(got) != 2 || got[1].BaseURL != "" || got[1].Model != "qwen" {
		t.Fatalf("substituição = %+v", got)
	}

	if err := svc.Delete("z-local"); err != nil {
		t.Fatal(err)
	}
	if got, _ = svc.Profiles(); len(got) != 1 {
		t.Fatalf("delete = %+v", got)
	}
	if err := svc.Delete("z-local"); err == nil {
		t.Error("delete de perfil inexistente deveria falhar")
	}
}

func TestSaveValidations(t *testing.T) {
	svc := New(nil, core.PathsIn(t.TempDir()))
	for _, p := range []agent.ProviderProfile{
		{Name: "  ", BaseURL: "https://x"},
		{Name: strings.Repeat("n", maxNameLen+1), BaseURL: "https://x"},
		{Name: "vazio"},
	} {
		if err := svc.Save(p); err == nil {
			t.Errorf("Save(%+v) deveria falhar", p)
		}
	}
}

func TestApplyStatusClear(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	claude := &fakeHost{id: "claude-code", installed: true}
	codex := &fakeHost{id: "codex", installed: true}
	off := &fakeHost{id: "gemini-cli"} // não instalado
	plain := &plainAdapter{id: "opencode"}
	svc := New([]agent.Adapter{claude, codex, off, plain}, paths)

	if err := svc.Save(agent.ProviderProfile{Name: "nuvem", BaseURL: "https://nuvem", Model: "m1", Token: "segredo"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply("nuvem", "claude-code"); err != nil {
		t.Fatal(err)
	}
	if claude.applied == nil || claude.applied.Token != "segredo" {
		t.Fatalf("apply não chegou no agente: %+v", claude.applied)
	}
	if claude.backups != paths.BackupsDir() {
		t.Errorf("backupsDir = %q", claude.backups)
	}
	if codex.applied != nil {
		t.Error("apply com agente nomeado não deveria tocar nos outros")
	}

	st := svc.Status()
	if len(st) != 3 {
		t.Fatalf("Status devolveu %d agentes; queria 3 (só os que suportam)", len(st))
	}
	if !st[0].Active || st[0].Profile != "nuvem" || st[0].AgentName != "CLAUDE-CODE" {
		t.Errorf("status do claude = %+v", st[0])
	}
	if st[0].Applied.Token != "" || !st[0].Applied.HasToken {
		t.Errorf("token vazou no Status: %+v", st[0].Applied)
	}
	if st[1].Active || st[2].Installed {
		t.Errorf("status dos demais = %+v", st[1:])
	}

	// sem agente: todos os instalados que suportam
	if err := svc.Apply("nuvem", ""); err != nil {
		t.Fatal(err)
	}
	if codex.applied == nil || off.applied != nil {
		t.Errorf("apply em todos: codex=%v gemini=%v", codex.applied != nil, off.applied != nil)
	}

	if err := svc.Clear(""); err != nil {
		t.Fatal(err)
	}
	if claude.applied != nil || codex.applied != nil {
		t.Error("clear não limpou")
	}
	if err := svc.Apply("nuvem", "inexistente"); err == nil {
		t.Error("agente inexistente deveria falhar")
	}
	if err := svc.Apply("outro", "claude-code"); err == nil {
		t.Error("perfil inexistente deveria falhar")
	}
}
