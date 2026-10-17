package hooks

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

type countedHookHost struct {
	agent.Adapter
	agent.HooksHost
	adds, removes       int
	failAdd, failRemove string
}

func (h *countedHookHost) AddHook(one agent.Hook, dir string) error {
	h.adds++
	if one.Command == h.failAdd {
		return errors.New("simulated command install failure")
	}
	return h.HooksHost.AddHook(one, dir)
}
func (h *countedHookHost) RemoveHook(one agent.Hook, dir string) error {
	h.removes++
	if one.Command == h.failRemove {
		return errors.New("simulated command removal failure")
	}
	return h.HooksHost.RemoveHook(one, dir)
}

func TestHookPartialCommandAndFailedOnlyRetry(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	claude, codex := agent.NewClaude(paths.Home), agent.NewCodex(paths.Home)
	good := &countedHookHost{Adapter: claude, HooksHost: claude}
	bad := &countedHookHost{Adapter: codex, HooksHost: codex, failAdd: "echo second"}
	svc := New([]agent.Adapter{good, bad, agent.NewPi(paths.Home)}, paths)
	svc.Detect = func() []agent.Agent {
		return []agent.Agent{{ID: good.ID(), Installed: true}, {ID: bad.ID(), Installed: true}}
	}
	h := Hook{Name: "pack", Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "echo first"}, {Event: agent.HookSessionStart, Command: "echo second"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := hooksToggle([]string{"pack"}, cli.Context{Out: &out, Err: &errOut}, svc, true); code != 1 {
		t.Fatalf("exit=%d", code)
	}
	for _, want := range []string{"claude-code: succeeded", "codex: failed", "pi: skipped", "1/2 command(s)", "--agent codex"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("missing %q: %s", want, &errOut)
		}
	}
	if out.Len() != 0 || good.adds != 2 || bad.adds != 2 {
		t.Fatal("unexpected partial writes")
	}
	bad.failAdd = ""
	r := svc.EnableResult("pack", []string{"codex"})
	if r.AsError() != nil || len(r.Targets) != 1 || good.adds != 2 || bad.adds != 4 {
		t.Fatalf("retry=%+v", r)
	}
	// The library flag changes before agent files. A failed command retry must
	// still run when that saved flag already matches the requested setting.
	bad.failRemove = "echo second"
	r = svc.SetCommandResult("pack", 1, false, nil)
	if r.AsError() == nil || !reflect.DeepEqual(r.FailedIDs(), []string{"codex"}) || good.removes != 1 {
		t.Fatalf("command result=%+v", r)
	}
	saved, _ := svc.Get("pack")
	if !saved.IsOff(1) {
		t.Fatal("library setting was not saved")
	}
	bad.failRemove = ""
	next := svc.SetCommandResult("pack", 1, false, r.FailedIDs())
	if next.AsError() != nil || good.removes != 1 || bad.removes != 2 {
		t.Fatalf("command retry=%+v", next)
	}
	r.MergeRetry(next)
	if r.AsError() != nil {
		t.Fatal(r.AsError())
	}
	if empty := svc.DisableResult("pack", r.FailedIDs()); len(empty.Targets) != 0 || good.removes != 1 {
		t.Fatal("empty retry repeated bulk removal")
	}
}
