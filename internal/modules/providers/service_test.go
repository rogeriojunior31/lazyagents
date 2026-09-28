package providers

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// fakeHost is an agent that supports providers, keeping its "file" in memory.
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

// plainAdapter does not support providers: it must stay out of Status.
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
		t.Fatalf("no file = %v, %v", got, err)
	}
	if err := svc.Save(agent.ProviderProfile{Name: "z-local", BaseURL: "http://localhost"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(agent.ProviderProfile{Name: "a-cloud", BaseURL: "https://cloud", Token: "secret"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(paths.ProvidersPath())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600 (may hold a token)", info.Mode().Perm())
	}

	got, err := svc.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a-cloud" { // alphabetical
		t.Fatalf("profiles = %+v", got)
	}
	if got[0].Token != "secret" {
		t.Errorf("token did not survive: %+v", got[0].Redacted())
	}

	// same name replaces, does not duplicate
	if err := svc.Save(agent.ProviderProfile{Name: "z-local", Model: "qwen"}); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Profiles()
	if len(got) != 2 || got[1].BaseURL != "" || got[1].Model != "qwen" {
		t.Fatalf("replace = %+v", got)
	}

	if err := svc.Delete("z-local"); err != nil {
		t.Fatal(err)
	}
	if got, _ = svc.Profiles(); len(got) != 1 {
		t.Fatalf("delete = %+v", got)
	}
	if err := svc.Delete("z-local"); err == nil {
		t.Error("deleting a missing profile should fail")
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
			t.Errorf("Save(%+v) should fail", p)
		}
	}
}

func TestApplyStatusClear(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	claude := &fakeHost{id: "claude-code", installed: true}
	codex := &fakeHost{id: "codex", installed: true}
	off := &fakeHost{id: "gemini-cli"} // not installed
	plain := &plainAdapter{id: "opencode"}
	svc := New([]agent.Adapter{claude, codex, off, plain}, paths)

	if err := svc.Save(agent.ProviderProfile{Name: "cloud", BaseURL: "https://cloud", Model: "m1", Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply("cloud", "claude-code"); err != nil {
		t.Fatal(err)
	}
	if claude.applied == nil || claude.applied.Token != "secret" {
		t.Fatalf("apply did not reach the agent: %+v", claude.applied)
	}
	if claude.backups != paths.BackupsDir() {
		t.Errorf("backupsDir = %q", claude.backups)
	}
	if codex.applied != nil {
		t.Error("apply with a named agent should not touch the others")
	}

	st := svc.Status()
	if len(st) != 3 {
		t.Fatalf("Status returned %d agents; want 3 (only those that support it)", len(st))
	}
	if !st[0].Active || st[0].Profile != "cloud" || st[0].AgentName != "CLAUDE-CODE" {
		t.Errorf("claude status = %+v", st[0])
	}
	if st[0].Applied.Token != "" || !st[0].Applied.HasToken {
		t.Errorf("token leaked into Status: %+v", st[0].Applied)
	}
	if st[1].Active || st[2].Installed {
		t.Errorf("other statuses = %+v", st[1:])
	}

	// no agent: every installed one that supports it
	if err := svc.Apply("cloud", ""); err != nil {
		t.Fatal(err)
	}
	if codex.applied == nil || off.applied != nil {
		t.Errorf("apply to all: codex=%v gemini=%v", codex.applied != nil, off.applied != nil)
	}

	if err := svc.Clear(""); err != nil {
		t.Fatal(err)
	}
	if claude.applied != nil || codex.applied != nil {
		t.Error("clear did not clear")
	}
	if err := svc.Apply("cloud", "missing"); err == nil {
		t.Error("missing agent should fail")
	}
	if err := svc.Apply("outro", "claude-code"); err == nil {
		t.Error("missing profile should fail")
	}
}
