package providers

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func validationService(t *testing.T) (*Service, *agent.Claude, *agent.Codex, core.Paths) {
	t.Helper()
	paths := core.PathsIn(t.TempDir())
	claude, codex := agent.NewClaude(paths.Home), agent.NewCodex(paths.Home)
	claude.ProviderState = filepath.Join(paths.DataDir, "claude-state.json")
	svc := New([]agent.Adapter{claude, codex}, paths)
	svc.Detect = func() []agent.Agent {
		return []agent.Agent{{ID: "claude-code", Name: "Claude Code", Installed: true}, {ID: "codex", Name: "Codex", Installed: true}}
	}
	return svc, claude, codex, paths
}

func TestValidationBlocksWholeSelectionWithoutSideEffects(t *testing.T) {
	for _, wire := range []string{"", "chat"} {
		t.Run("wire="+wire, func(t *testing.T) {
			svc, claude, codex, paths := validationService(t)
			p := agent.ProviderProfile{Name: "proxy", BaseURL: "https://example.test", Token: "secret-never-display", WireAPI: wire}
			if wire != "" {
				p.EnvKey = "TEST_TOKEN"
			}
			if err := svc.Save(p); err != nil {
				t.Fatal(err)
			}
			plan := svc.PrepareSaved(p.Name, nil)
			if plan.valid() || len(plan.Targets) != 2 || plan.Targets[0].Error != "" || plan.Targets[1].Error == "" {
				t.Fatalf("plan = %+v", plan)
			}
			data, _ := json.Marshal(plan)
			if strings.Contains(string(data), p.Token) || bytes.Contains(data, []byte(`"token"`)) {
				t.Fatalf("secret in plan: %s", data)
			}
			result := svc.ApplyResult(p.Name, nil)
			if result.AsError() == nil || len(result.FailedIDs()) != 2 {
				t.Fatalf("result = %+v", result)
			}
			for _, path := range []string{claude.ProviderFile(), codex.ProviderFile(), claude.ProviderState, paths.BackupsDir()} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("validation created %s: %v", path, err)
				}
			}
			// A valid subset is used only when the caller explicitly chooses it.
			if err := svc.Apply(p.Name, "claude-code"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(codex.ProviderFile()); !os.IsNotExist(err) {
				t.Fatal("subset touched Codex")
			}
		})
	}
}

func TestMalformedNativeConfigBlocksBeforeAnotherTargetWrites(t *testing.T) {
	svc, claude, codex, paths := validationService(t)
	if err := svc.Save(agent.ProviderProfile{Name: "local", BaseURL: "http://localhost:1234", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(codex.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("[model_providers.lazyagents]\nbase_url = \"https://foreign.test\"\n")
	if err := os.WriteFile(codex.ProviderFile(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	r := svc.ApplyResult("local", nil)
	if r.AsError() == nil {
		t.Fatal("foreign table was accepted")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("Claude written before validation finished")
	}
	got, _ := os.ReadFile(codex.ProviderFile())
	if !bytes.Equal(got, original) {
		t.Fatal("foreign Codex config changed")
	}
	if _, err := os.Stat(paths.BackupsDir()); !os.IsNotExist(err) {
		t.Fatal("preflight made backups")
	}
}

func TestValidationCLIAndExplicitSelection(t *testing.T) {
	svc, claude, codex, _ := validationService(t)
	if err := svc.Save(agent.ProviderProfile{Name: "proxy", BaseURL: "https://example.test", Token: "secret-never-display"}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	c := cli.Context{Out: &out, Err: &errOut}
	if code := cmdProvider([]string{"validate", "proxy", "--json"}, c, svc); code != 1 {
		t.Fatalf("validate exit %d", code)
	}
	var plan Preparation
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil || plan.valid() {
		t.Fatalf("JSON = %s: %v", &out, err)
	}
	if strings.Contains(out.String()+errOut.String(), "secret-never-display") {
		t.Fatal("token exposed")
	}
	out.Reset()
	errOut.Reset()
	if code := providerApply([]string{"proxy"}, c, svc); code != 1 || !strings.Contains(errOut.String(), "No files changed") || out.Len() != 0 {
		t.Fatalf("apply: %d %s %s", code, &out, &errOut)
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("failed bulk changed Claude")
	}
	out.Reset()
	errOut.Reset()
	if code := providerApply([]string{"proxy", "--agents", "claude-code,claude-code"}, c, svc); code != 0 {
		t.Fatalf("subset: %d %s", code, &errOut)
	}
	if _, err := os.Stat(codex.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("subset changed Codex")
	}
	for _, args := range [][]string{{"proxy", "--agents", ""}, {"proxy", "--agent", "claude-code", "--agents", "codex"}, {"proxy", "--agents", "claude-code,"}, {"proxy", "--agents", "missing"}} {
		out.Reset()
		errOut.Reset()
		if code := providerApply(args, c, svc); code != 1 {
			t.Fatalf("invalid selection accepted: %v", args)
		}
	}
}

func TestPickerRequiresExplicitCompatibleSubset(t *testing.T) {
	svc, claude, codex, _ := validationService(t)
	if err := svc.Save(agent.ProviderProfile{Name: "proxy", BaseURL: "https://example.test", Token: "secret-never-display"}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"}))
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm != nil || m.targets == nil || m.targets.err == "" {
		t.Fatal("invalid selection reached confirmation")
	}
	for _, size := range [][2]int{{40, 8}, {40, 14}, {80, 24}, {120, 32}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		v := m.View()
		if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] || !strings.Contains(ansi.Strip(v), "esc") || strings.Contains(v, "secret-never-display") {
			t.Fatalf("picker at %v:\n%s", size, ansi.Strip(v))
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // explicitly exclude Codex
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil || strings.Contains(m.confirm.Question, "\nCodex\n") {
		t.Fatal("subset confirmation is incorrect")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.report == nil || m.report.Result.AsError() != nil {
		t.Fatalf("subset failed: %+v", m.report)
	}
	if _, err := os.Stat(claude.ProviderFile()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(codex.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("unselected target written")
	}
}

func TestDraftValidationDoesNotSaveAndCancelledCheckStaysClosed(t *testing.T) {
	svc, claude, _, _ := validationService(t)
	if err := svc.Save(agent.ProviderProfile{Name: "proxy", BaseURL: "https://example.test", Token: "secret-never-display"}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	before, _ := os.ReadFile(svc.Path())
	key := tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}
	run(t, &m, m.Update(key))
	if m.targets == nil || !m.targets.inspect || m.targets.plan.Targets[1].Error == "" {
		t.Fatal("draft did not validate saved token presence")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.form == nil {
		t.Fatal("check discarded draft")
	}
	cmd := m.Update(key)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	run(t, &m, cmd)
	if m.targets != nil || m.preparing {
		t.Fatal("cancelled check reopened")
	}
	after, _ := os.ReadFile(svc.Path())
	if !bytes.Equal(before, after) {
		t.Fatal("check saved profile")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("check applied profile")
	}
}

func TestApplyRevalidatesFilesAfterConfirmationReview(t *testing.T) {
	svc, claude, codex, _ := validationService(t)
	if err := svc.Save(agent.ProviderProfile{Name: "local", BaseURL: "http://localhost:1234", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"}))
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil {
		t.Fatal("valid selection did not reach confirmation")
	}
	if err := os.MkdirAll(filepath.Dir(codex.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("# lazyagents — managed block end\n")
	if err := os.WriteFile(codex.ProviderFile(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.report == nil || m.report.Result.AsError() == nil {
		t.Fatal("apply trusted the stale review")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("stale review changed Claude before validating Codex")
	}
	after, _ := os.ReadFile(codex.ProviderFile())
	if !bytes.Equal(after, original) {
		t.Fatal("stale review overwrote current Codex config")
	}
}
