package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func testService(t *testing.T) (*Service, string) {
	t.Helper()
	home := t.TempDir()
	// Claude Code counts as installed when ~/.claude exists.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{agent.NewClaude(home)}, core.PathsIn(home))
	return svc, home
}

func TestLibraryRoundTrip(t *testing.T) {
	svc, _ := testService(t)
	if lib, problems := svc.Library(); lib != nil || problems != nil {
		t.Fatalf("empty library = %v, %v", lib, problems)
	}
	h := Hook{Name: "doctor", Description: "runs doctor on start",
		Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "lazyagents doctor", Timeout: 5}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(Hook{Name: "b", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo end"}}}); err != nil {
		t.Fatal(err)
	}
	lib, problems := svc.Library()
	if len(lib) != 2 || len(problems) != 0 || lib[0].Name != "b" { // alphabetical
		t.Fatalf("Library = %+v, %v", lib, problems)
	}
	got, err := svc.Get("doctor")
	if err != nil || len(got.Hooks) != 1 || got.Hooks[0].Command != "lazyagents doctor" || got.Hooks[0].Timeout != 5 || got.Description == "" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	// A broken file is its own problem and does not fail the listing.
	if err := os.WriteFile(filepath.Join(svc.Dir(), "bad.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	lib, problems = svc.Library()
	if len(lib) != 2 || len(problems) != 1 || !strings.Contains(problems[0], "bad.json") {
		t.Fatalf("Library with a bad file = %+v, %v", lib, problems)
	}

	if err := svc.Delete("doctor"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get("doctor"); err == nil {
		t.Error("Get after Delete should fail")
	}
	if err := svc.Delete("doctor"); err == nil {
		t.Error("Delete of a missing hook should fail")
	}
}

func TestSaveValidations(t *testing.T) {
	svc, _ := testService(t)
	for _, h := range []Hook{
		{Name: "", Hooks: []agent.Hook{{Event: "Stop", Command: "x"}}},
		{Name: "../escape", Hooks: []agent.Hook{{Event: "Stop", Command: "x"}}},
		{Name: strings.Repeat("n", maxNameLen+1), Hooks: []agent.Hook{{Event: "Stop", Command: "x"}}},
		{Name: "no-command", Hooks: []agent.Hook{{Event: "Stop"}}},
		{Name: "no-event", Hooks: []agent.Hook{{Command: "x"}}},
	} {
		if err := svc.Save(h); err == nil {
			t.Errorf("Save(%+v) should fail", h)
		}
	}
}

func TestEnableDisableAndForeignHooks(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)

	// The user's own hook, which lazyagents must not touch.
	foreign := agent.Hook{Event: agent.HookStop, Command: "my-script.sh"}
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
		t.Errorf("incomplete status = %+v", st[0])
	}

	if err := svc.Disable("doctor", "claude-code"); err != nil {
		t.Fatal(err)
	}
	st = svc.Status()
	if len(st[0].Enabled) != 0 || st[0].Foreign != 1 {
		t.Errorf("disable = %+v", st[0])
	}
	// The foreign hook is still there.
	hooks, err := claude.ReadHooks()
	if err != nil || len(hooks) != 1 || !hooks[0].Same(foreign) {
		t.Errorf("user's hook was touched: %+v, %v", hooks, err)
	}
}

// Installing a hook for an event the agent never fires is an error, not a no-op.
func TestEnableRefusesUnsupportedEvent(t *testing.T) {
	svc, home := testService(t)
	codex := agent.NewCodex(home)
	svc.adapters = append(svc.adapters, codex)

	h := Hook{Name: "stop", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo x"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	err := svc.Enable("stop", "codex") // Codex does not fire Stop
	if err == nil || !strings.Contains(err.Error(), "does not fire") {
		t.Errorf("Enable on codex = %v", err)
	}
	if err := svc.Enable("missing", "claude-code"); err == nil {
		t.Error("a hook outside the library should fail")
	}
}

func TestCommandProblem(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "x.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"":                                  "empty command",
		"command-that-does-not-exist-xyzzy": "PATH",
		filepath.Join(dir, "gone.sh"):       "not found",
	}
	if runtime.GOOS != "windows" { // Windows has no exec bit
		cases[script] = "not executable"
	}
	for cmd, want := range cases {
		if got := CommandProblem(Hook{Hooks: []agent.Hook{{Command: cmd}}}); !strings.Contains(got, want) {
			t.Errorf("CommandProblem(%q) = %q, want it to contain %q", cmd, got, want)
		}
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := CommandProblem(Hook{Hooks: []agent.Hook{{Command: script + " session"}}}); got != "" {
		t.Errorf("executable script = %q", got)
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

// A turned-off command is not installed; toggling with the package installed
// applies to the agent right away.
func TestCommandOffAndSetCommand(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	a := agent.Hook{Event: agent.HookSessionStart, Command: "echo a"}
	b := agent.Hook{Event: agent.HookStop, Command: "echo b"}
	if err := svc.Save(Hook{Name: "pack", Hooks: []agent.Hook{a, b}, Off: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Get("pack"); len(got.Off) != 1 || !got.IsOff(1) {
		t.Fatalf("normalized Off = %v", got.Off)
	}
	if err := svc.Save(Hook{Name: "bad", Hooks: []agent.Hook{a}, Off: []int{3}}); err == nil {
		t.Error("out-of-range Off should fail")
	}

	if err := svc.Enable("pack", "claude-code"); err != nil {
		t.Fatal(err)
	}
	installed, _ := claude.ReadHooks()
	if !containsHook(installed, a) || containsHook(installed, b) {
		t.Fatalf("installed the turned-off command: %+v", installed)
	}
	if st := svc.Status(); len(st[0].Enabled) != 1 {
		t.Errorf("package with its active command installed should be complete: %+v", st[0])
	}

	// Turn b on: the package is in the agent, so b goes there.
	if err := svc.SetCommand("pack", 1, true); err != nil {
		t.Fatal(err)
	}
	installed, _ = claude.ReadHooks()
	if !containsHook(installed, b) {
		t.Errorf("turning on did not install: %+v", installed)
	}
	// Turn a off: it leaves the agent and the count.
	if err := svc.SetCommand("pack", 0, false); err != nil {
		t.Fatal(err)
	}
	installed, _ = claude.ReadHooks()
	if containsHook(installed, a) || !containsHook(installed, b) {
		t.Errorf("turning off did not remove: %+v", installed)
	}
	if st := svc.Status(); len(st[0].Enabled) != 1 || st[0].Foreign != 0 {
		t.Errorf("status after toggle = %+v", st[0])
	}

	// Everything off: enable refuses.
	if err := svc.SetCommand("pack", 1, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enable("pack", ""); err == nil || !strings.Contains(err.Error(), "turned off") {
		t.Errorf("Enable with no active command = %v", err)
	}
}

// Outside the agents, turning off changes only the library.
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
		t.Error("wrote to the agent without the package installed")
	}
	if h, _ := svc.Get("pack"); !h.IsOff(0) || h.Summary() != "1 of 2 commands in 1 events" {
		t.Errorf("entry = %+v / %q", h, h.Summary())
	}
	if err := svc.SetCommand("pack", 5, false); err == nil {
		t.Error("a missing index should fail")
	}
}
