package providers

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func partialService(t *testing.T) (*Service, *agent.Claude, core.Paths) {
	t.Helper()
	paths := core.PathsIn(t.TempDir())
	claude, codex := agent.NewClaude(paths.Home), agent.NewCodex(paths.Home)
	svc := New([]agent.Adapter{claude, codex, &fakeHost{id: "offline"}, &plainAdapter{id: "unsupported"}}, paths)
	svc.Detect = func() []agent.Agent {
		return []agent.Agent{{ID: "claude-code", Name: "Claude Code", Installed: true}, {ID: "codex", Name: "Codex", Installed: true}}
	}
	if err := svc.Save(agent.ProviderProfile{Name: "partial", BaseURL: "https://example.test/v1", Token: "secret-never-display"}); err != nil {
		t.Fatal(err)
	}
	return svc, claude, paths
}

func TestPartialResultAndFailedOnlyRetry(t *testing.T) {
	svc, claude, paths := partialService(t)
	r := svc.ApplyResult("partial", nil)
	if r.AsError() == nil || !reflect.DeepEqual(r.FailedIDs(), []string{"codex"}) {
		t.Fatalf("result = %+v", r)
	}
	want := []core.TargetOutcome{core.Succeeded, core.Failed, core.Skipped, core.Skipped}
	for i, tgt := range r.Targets {
		if tgt.Outcome != want[i] {
			t.Fatalf("target = %+v", tgt)
		}
	}
	if !strings.Contains(r.Targets[0].State, `"partial"`) || r.Targets[1].State != "agent default" {
		t.Fatalf("state = %+v", r.Targets)
	}
	before, err := os.Stat(claude.ProviderFile())
	if err != nil {
		t.Fatal(err)
	}
	p, _ := svc.Profile("partial")
	p.EnvKey = "TEST_TOKEN"
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	next := svc.ApplyResult("partial", r.FailedIDs())
	if next.AsError() != nil || len(next.Targets) != 1 || next.Targets[0].ID != "codex" {
		t.Fatalf("retry = %+v", next)
	}
	after, _ := os.Stat(claude.ProviderFile())
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("successful target rewritten on retry")
	}
	backups, _ := filepath.Glob(filepath.Join(paths.BackupsDir(), "*"))
	if len(backups) != 0 {
		t.Fatalf("successful target backed up again: %v", backups)
	}
	r.MergeRetry(next)
	if r.AsError() != nil || len(r.Targets) != 4 || len(r.FailedIDs()) != 0 {
		t.Fatalf("merged = %+v", r)
	}
	if empty := svc.ApplyResult("partial", r.FailedIDs()); len(empty.Targets) != 0 || empty.AsError() != nil {
		t.Fatalf("empty retry = %+v", empty)
	}
}

func TestPartialCLIAndPersistentReport(t *testing.T) {
	svc, claude, _ := partialService(t)
	var out, errOut bytes.Buffer
	if code := providerApply([]string{"partial"}, cli.Context{Out: &out, Err: &errOut}, svc); code != 1 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"claude-code: succeeded", "codex: failed", "offline: skipped", "--agent codex", "config-backups"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("missing %q: %s", want, &errOut)
		}
	}
	if out.Len() != 0 || strings.Contains(errOut.String(), "secret-never-display") {
		t.Fatal("false success or exposed token")
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.report == nil || !m.Capturing() || !m.statuses[0].Active || m.statuses[1].Active {
		t.Fatalf("report/state = %+v %+v", m.report, m.statuses)
	}
	m.ClearToast()
	for _, w := range []int{100, 40} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 32})
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "claude-code: succeeded") || !strings.Contains(view, "codex: failed") || strings.Contains(view, "secret-never-display") {
			t.Fatalf("report:\n%s", view)
		}
	}
	// Retry is confirmed and rereads the edited saved profile, without touching Claude.
	p, _ := svc.Profile("partial")
	p.EnvKey = "TEST_TOKEN"
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(claude.ProviderFile())
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if m.report == nil || !strings.Contains(m.View(), "Retry only failed targets: codex") {
		t.Fatal("retry must name failed target")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	after, _ := os.Stat(claude.ProviderFile())
	if len(m.report.Result.FailedIDs()) != 0 || !before.ModTime().Equal(after.ModTime()) || !m.statuses[1].Active {
		t.Fatal("retry did not preserve success and refresh state")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.report != nil || m.Capturing() {
		t.Fatal("report not dismissed")
	}
}
