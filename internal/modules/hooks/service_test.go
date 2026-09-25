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
		Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "lazyagents doctor", Timeout: 5}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(Hook{Name: "b", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo fim"}}}); err != nil {
		t.Fatal(err)
	}
	lib, problems := svc.Library()
	if len(lib) != 2 || len(problems) != 0 || lib[0].Name != "b" { // ordem alfabética
		t.Fatalf("Library = %+v, %v", lib, problems)
	}
	got, err := svc.Get("doctor")
	if err != nil || len(got.Hooks) != 1 || got.Hooks[0].Command != "lazyagents doctor" || got.Hooks[0].Timeout != 5 || got.Description == "" {
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
		{Name: "", Hooks: []agent.Hook{{Event: "Stop", Command: "x"}}},
		{Name: "../fuga", Hooks: []agent.Hook{{Event: "Stop", Command: "x"}}},
		{Name: strings.Repeat("n", maxNameLen+1), Hooks: []agent.Hook{{Event: "Stop", Command: "x"}}},
		{Name: "sem-comando", Hooks: []agent.Hook{{Event: "Stop"}}},
		{Name: "sem-evento", Hooks: []agent.Hook{{Command: "x"}}},
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

	h := Hook{Name: "doctor", Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "lazyagents doctor"}}}
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

	h := Hook{Name: "parada", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo x"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	err := svc.Enable("parada", "codex") // o Codex não dispara Stop
	if err == nil || !strings.Contains(err.Error(), "does not fire") {
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
		"":                             "empty command",
		"comando-que-nao-existe-xyzzy": "PATH",
		script:                         "not executable",
		filepath.Join(dir, "sumiu.sh"): "not found",
	}
	for cmd, want := range cases {
		if got := CommandProblem(Hook{Hooks: []agent.Hook{{Command: cmd}}}); !strings.Contains(got, want) {
			t.Errorf("CommandProblem(%q) = %q, queria conter %q", cmd, got, want)
		}
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := CommandProblem(Hook{Hooks: []agent.Hook{{Command: script + " session"}}}); got != "" {
		t.Errorf("script executável = %q", got)
	}
}

func TestDeleteRefusesExternalScripts(t *testing.T) {
	svc, _ := testService(t)
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	h := Hook{Name: "external", Files: outside, Hooks: []agent.Hook{{Event: "Stop", Command: "true"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(h.Name); err == nil {
		t.Fatal("accepted outside directory")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(h.Name); err != nil {
		t.Fatal("entry lost", err)
	}
}

// Comando desligado não é instalado, e ligar/desligar com o pacote já
// instalado vale na hora no agente.
func TestCommandOffAndSetCommand(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	a := agent.Hook{Event: agent.HookSessionStart, Command: "echo a"}
	b := agent.Hook{Event: agent.HookStop, Command: "echo b"}
	if err := svc.Save(Hook{Name: "pack", Hooks: []agent.Hook{a, b}, Off: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Get("pack"); len(got.Off) != 1 || !got.IsOff(1) {
		t.Fatalf("Off normalizado = %v", got.Off)
	}
	if err := svc.Save(Hook{Name: "ruim", Hooks: []agent.Hook{a}, Off: []int{3}}); err == nil {
		t.Error("Off fora do intervalo deveria falhar")
	}

	if err := svc.Enable("pack", "claude-code"); err != nil {
		t.Fatal(err)
	}
	installed, _ := claude.ReadHooks()
	if !containsHook(installed, a) || containsHook(installed, b) {
		t.Fatalf("instalou o desligado: %+v", installed)
	}
	if st := svc.Status(); len(st[0].Enabled) != 1 {
		t.Errorf("pacote com o ligado instalado deveria estar inteiro: %+v", st[0])
	}

	// Liga b: o pacote está no agente, então b entra lá.
	if err := svc.SetCommand("pack", 1, true); err != nil {
		t.Fatal(err)
	}
	installed, _ = claude.ReadHooks()
	if !containsHook(installed, b) {
		t.Errorf("ligar não instalou: %+v", installed)
	}
	// Desliga a: sai do agente e da contagem.
	if err := svc.SetCommand("pack", 0, false); err != nil {
		t.Fatal(err)
	}
	installed, _ = claude.ReadHooks()
	if containsHook(installed, a) || !containsHook(installed, b) {
		t.Errorf("desligar não removeu: %+v", installed)
	}
	if st := svc.Status(); len(st[0].Enabled) != 1 || st[0].Foreign != 0 {
		t.Errorf("status depois do toggle = %+v", st[0])
	}

	// Desliga tudo: enable recusa.
	if err := svc.SetCommand("pack", 1, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enable("pack", ""); err == nil || !strings.Contains(err.Error(), "turned off") {
		t.Errorf("Enable sem comando ligado = %v", err)
	}
}

// Fora dos agentes, desligar muda só a biblioteca.
func TestSetCommandLibraryOnly(t *testing.T) {
	svc, home := testService(t)
	if err := svc.Save(Hook{Name: "pack", Hooks: []agent.Hook{
		{Event: agent.HookStop, Command: "echo a"}, {Event: agent.HookStop, Command: "echo b"}}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCommand("pack", 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(agent.NewClaude(home).HooksFile()); !os.IsNotExist(err) {
		t.Error("escreveu no agente sem o pacote instalado")
	}
	if h, _ := svc.Get("pack"); !h.IsOff(0) || h.Summary() != "1 of 2 commands in 1 events" {
		t.Errorf("entrada = %+v / %q", h, h.Summary())
	}
	if err := svc.SetCommand("pack", 5, false); err == nil {
		t.Error("índice inexistente deveria falhar")
	}
}
