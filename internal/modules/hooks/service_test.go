package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func testService(t *testing.T) (*Service, string) {
	t.Helper()
	home := t.TempDir()
	// O Claude Code é detectado como instalado quando o ~/.claude existe.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{agent.NewClaude(home)}, core.PathsIn(home))
	return svc, home
}

func TestLibraryRoundTrip(t *testing.T) {
	svc, _ := testService(t)
	if lib, problems := svc.Library(); lib != nil || problems != nil {
		t.Fatalf("biblioteca vazia = %v, %v", lib, problems)
	}
	h := Hook{Name: "doctor", Description: "roda o doctor ao abrir",
		Hook: agent.Hook{Event: agent.HookSessionStart, Command: "lazyagents doctor", Timeout: 5}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(Hook{Name: "b", Hook: agent.Hook{Event: agent.HookStop, Command: "echo fim"}}); err != nil {
		t.Fatal(err)
	}
	lib, problems := svc.Library()
	if len(lib) != 2 || len(problems) != 0 || lib[0].Name != "b" { // ordem alfabética
		t.Fatalf("Library = %+v, %v", lib, problems)
	}
	got, err := svc.Get("doctor")
	if err != nil || got.Command != "lazyagents doctor" || got.Timeout != 5 || got.Description == "" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	// Arquivo quebrado vira problema só dele, sem derrubar a listagem.
	if err := os.WriteFile(filepath.Join(svc.Dir(), "ruim.json"), []byte("{não é json"), 0o600); err != nil {
		t.Fatal(err)
	}
	lib, problems = svc.Library()
	if len(lib) != 2 || len(problems) != 1 || !strings.Contains(problems[0], "ruim.json") {
		t.Fatalf("Library com arquivo ruim = %+v, %v", lib, problems)
	}

	if err := svc.Delete("doctor"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get("doctor"); err == nil {
		t.Error("Get depois do Delete deveria falhar")
	}
	if err := svc.Delete("doctor"); err == nil {
		t.Error("Delete de hook inexistente deveria falhar")
	}
}

func TestSaveValidations(t *testing.T) {
	svc, _ := testService(t)
	for _, h := range []Hook{
		{Name: "", Hook: agent.Hook{Event: "Stop", Command: "x"}},
		{Name: "../fuga", Hook: agent.Hook{Event: "Stop", Command: "x"}},
		{Name: strings.Repeat("n", maxNameLen+1), Hook: agent.Hook{Event: "Stop", Command: "x"}},
		{Name: "sem-comando", Hook: agent.Hook{Event: "Stop"}},
		{Name: "sem-evento", Hook: agent.Hook{Command: "x"}},
	} {
		if err := svc.Save(h); err == nil {
			t.Errorf("Save(%+v) deveria falhar", h)
		}
	}
}

func TestEnableDisableAndForeignHooks(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)

	// Hook do usuário, que o lazyagents não pode tocar.
	foreign := agent.Hook{Event: agent.HookStop, Command: "meu-script.sh"}
	if err := claude.AddHook(foreign, ""); err != nil {
		t.Fatal(err)
	}

	h := Hook{Name: "doctor", Hook: agent.Hook{Event: agent.HookSessionStart, Command: "lazyagents doctor"}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enable("doctor", "claude-code"); err != nil {
		t.Fatal(err)
	}

	st := svc.Status()
	if len(st) != 1 {
		t.Fatalf("Status = %+v", st)
	}
	if len(st[0].Enabled) != 1 || st[0].Enabled[0] != "doctor" || st[0].Foreign != 1 {
		t.Errorf("status = %+v", st[0])
	}
	if !st[0].Installed || st[0].File == "" || len(st[0].Events) == 0 {
		t.Errorf("status incompleto = %+v", st[0])
	}

	if err := svc.Disable("doctor", "claude-code"); err != nil {
		t.Fatal(err)
	}
	st = svc.Status()
	if len(st[0].Enabled) != 0 || st[0].Foreign != 1 {
		t.Errorf("disable = %+v", st[0])
	}
	// O hook alheio continua lá.
	hooks, err := claude.ReadHooks()
	if err != nil || len(hooks) != 1 || !hooks[0].Same(foreign) {
		t.Errorf("hook do usuário foi mexido: %+v, %v", hooks, err)
	}
}

// Instalar um hook de evento que o agente não dispara é erro, não silêncio.
func TestEnableRefusesUnsupportedEvent(t *testing.T) {
	svc, home := testService(t)
	codex := agent.NewCodex(home)
	svc.adapters = append(svc.adapters, codex)

	h := Hook{Name: "parada", Hook: agent.Hook{Event: agent.HookStop, Command: "echo x"}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	err := svc.Enable("parada", "codex") // o Codex não dispara Stop
	if err == nil || !strings.Contains(err.Error(), "não dispara") {
		t.Errorf("Enable no codex = %v", err)
	}
	if err := svc.Enable("inexistente", "claude-code"); err == nil {
		t.Error("hook fora da biblioteca deveria falhar")
	}
}

func TestCommandProblem(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "x.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"":                             "comando vazio",
		"comando-que-nao-existe-xyzzy": "PATH",
		script:                         "permissão",
		filepath.Join(dir, "sumiu.sh"): "não encontrado",
	}
	for cmd, want := range cases {
		if got := CommandProblem(Hook{Hook: agent.Hook{Command: cmd}}); !strings.Contains(got, want) {
			t.Errorf("CommandProblem(%q) = %q, queria conter %q", cmd, got, want)
		}
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := CommandProblem(Hook{Hook: agent.Hook{Command: script + " session"}}); got != "" {
		t.Errorf("script executável = %q", got)
	}
}
