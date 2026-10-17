package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

func TestSkillPartialResultsAndRetry(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "library"))
	good := testAgent(p.Home, "good", "good/skills")
	bad := testAgent(p.Home, "bad", "bad/skills")
	local := testAgent(p.Home, "local", "local/skills")
	writeSkill(t, local.ManagedDir, "sk", validMD("sk", "local"))
	if err := os.MkdirAll(filepath.Dir(bad.ManagedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad.ManagedDir, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	agents := []agent.Agent{good, bad, local, {ID: "unsupported", Installed: true}, {ID: "offline"}}
	r := svc.ToggleResult("sk", true, agents, nil)
	if r.AsError() == nil || !reflect.DeepEqual(r.FailedIDs(), []string{"bad"}) {
		t.Fatalf("result = %+v", r)
	}
	if r.Targets[0].Outcome != core.Succeeded || r.Targets[2].Outcome != core.Skipped || r.Targets[2].State != "local; preserved" {
		t.Fatalf("targets = %+v", r.Targets)
	}
	before, err := os.Lstat(filepath.Join(good.ManagedDir, "sk"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bad.ManagedDir); err != nil {
		t.Fatal(err)
	}
	next := svc.ToggleResult("sk", true, agents, r.FailedIDs())
	if next.AsError() != nil || len(next.Targets) != 1 {
		t.Fatalf("retry = %+v", next)
	}
	after, _ := os.Lstat(filepath.Join(good.ManagedDir, "sk"))
	if !os.SameFile(before, after) {
		t.Fatal("successful link replaced on retry")
	}
	r.MergeRetry(next)
	if r.AsError() != nil || len(r.FailedIDs()) != 0 {
		t.Fatalf("merged = %+v", r)
	}
	empty := svc.ToggleResult("sk", false, agents, r.FailedIDs())
	if empty.AsError() != nil || len(empty.Targets) != 0 || !scanOne(t, svc, agents, "sk").States[good.ID].On {
		t.Fatal("empty retry changed successful targets")
	}
	// The persistent panel resists toast expiry, and the matrix is rescanned.
	m := newTab(svc)
	m.agents, m.targets = agents, skillCapableAgents(agents)
	m.width, m.height = 80, 25
	m, cmd := m.update(bulkMsg{name: "sk", enable: true, result: r})
	if m.report == nil || !m.Capturing() {
		t.Fatal("report not capturing")
	}
	m, _ = m.update(cmd())
	m, _ = m.update(skToastExpire{seq: m.toastSeq})
	if !strings.Contains(m.View(), "good: succeeded") {
		t.Fatal("report expired")
	}
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.report != nil || m.Capturing() {
		t.Fatal("report did not dismiss")
	}
}

func TestProfileSharedFailureAndRetry(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "d"))
	a, b, c := testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills"), testAgent(p.Home, "c", "c/skills")
	shared := filepath.Join(p.Home, "shared")
	a.SharedDir, b.SharedDir = shared, shared
	a.ReadDirs, b.ReadDirs = append(a.ReadDirs, shared), append(b.ReadDirs, shared)
	agents := []agent.Agent{a, b, c}
	if err := svc.EnableAll(scanOne(t, svc, agents, "sk"), agents); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Lstat(filepath.Join(c.ManagedDir, "sk"))
	// Moving A out of the shared link fails; B remains visible until the retry.
	if err := os.MkdirAll(filepath.Dir(a.ManagedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ManagedDir, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveProfile("work", ProfileSpec{"sk": {a.ID, c.ID}}); err != nil {
		t.Fatal(err)
	}
	r := svc.ApplyProfileResult("work", agents, nil)
	if r.AsError() == nil || !reflect.DeepEqual(r.FailedIDs(), []string{"a", "b"}) || r.Targets[2].Outcome != core.Succeeded {
		t.Fatalf("result = %+v", r)
	}
	if !scanOne(t, svc, agents, "sk").States[b.ID].On {
		t.Fatal("old shared link lost on failure")
	}
	if err := os.Remove(a.ManagedDir); err != nil {
		t.Fatal(err)
	}
	next := svc.ApplyProfileResult("work", agents, r.FailedIDs())
	if next.AsError() != nil {
		t.Fatalf("retry = %+v", next)
	}
	state := scanOne(t, svc, agents, "sk")
	after, _ := os.Lstat(filepath.Join(c.ManagedDir, "sk"))
	if !state.States[a.ID].On || state.States[b.ID].On || !state.States[c.ID].On || !os.SameFile(before, after) {
		t.Fatalf("retry state = %+v", state.States)
	}
	// A completion for another report cannot replace this tab's result.
	m := newTab(svc)
	m.report = &components.OperationReport{Result: r}
	m, _ = m.update(components.OperationFinished{Report: &components.OperationReport{}, Result: next})
	if !reflect.DeepEqual(m.report.Result.FailedIDs(), []string{"a", "b"}) {
		t.Fatal("foreign completion replaced result")
	}
}
