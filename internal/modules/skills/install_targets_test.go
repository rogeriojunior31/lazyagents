package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

func installTarget(t *testing.T, r core.OperationResult, id string) core.TargetResult {
	t.Helper()
	for _, target := range r.Targets {
		if target.ID == id {
			return target
		}
	}
	t.Fatalf("missing %s in %+v", id, r)
	return core.TargetResult{}
}

func TestInstallSubsetAndRetryOnlyActivation(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	source := t.TempDir()
	for _, name := range []string{"new", "existing", "hidden"} {
		writeSkill(t, source, name, validMD(name, "source"))
	}
	hooksDir := filepath.Join(source, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo fixture"}]}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, p.LibraryDir(), "existing", validMD("existing", "original"))
	found, origin, cleanup, err := svc.Discover(source)
	if err != nil || cleanup != "" {
		t.Fatalf("discover: %s %v", cleanup, err)
	}
	chosen := slices.DeleteFunc(slices.Clone(found), func(f Found) bool { return f.Name == "hidden" })
	a, b := testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills")
	agents := []agent.Agent{a, b}
	if err := os.MkdirAll(filepath.Dir(b.ManagedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.ManagedDir, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := svc.installWithTargets(chosen, origin, agents, []string{"a", "b"})
	if installTarget(t, out.result, "new / install").Outcome != core.Succeeded || installTarget(t, out.result, "new / b").Outcome != core.Failed || installTarget(t, out.result, "existing / install").Outcome != core.Failed {
		t.Fatalf("result: %+v", out.result)
	}
	if scanOne(t, svc, agents, "existing").EnabledCount() != 0 || scanOne(t, svc, agents, "existing").Description != "original" {
		t.Fatal("collision changed or activated the existing library copy")
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "hidden")); !os.IsNotExist(err) {
		t.Fatal("unselected skill installed")
	}
	entries, err := filepath.Glob(filepath.Join(p.HooksDir(), "*.json"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("hook import: %v %v", entries, err)
	}
	before, _ := os.Lstat(filepath.Join(a.ManagedDir, "new"))
	writeSkill(t, p.LibraryDir(), "new", validMD("new", "edited after install"))
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(b.ManagedDir); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.finishInstall(out)
	m.report.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, 100, 24)
	if v := m.report.View(100, 24); !strings.Contains(v, "new / b") || strings.Contains(v, "existing / install") {
		t.Fatalf("retry scope: %s", v)
	}
	_, cmd := m.report.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}, 100, 24)
	m.report.Finish(cmd().(components.OperationFinished))
	if !reflect.DeepEqual(m.report.Result.FailedIDs(), []string{"existing / install"}) {
		t.Fatalf("retry lost outcomes: %+v", m.report.Result)
	}
	after, _ := os.Lstat(filepath.Join(a.ManagedDir, "new"))
	if !os.SameFile(before, after) || !scanOne(t, svc, agents, "new").States[b.ID].On || scanOne(t, svc, agents, "new").Description != "edited after install" {
		t.Fatal("retry replaced installed files or a successful activation")
	}
	if empty := svc.retryInstall(out, []string{}); len(empty.Targets) != 0 {
		t.Fatal("empty retry expanded")
	}
	if ids, err := svc.installTargets(); err != nil || !slices.Equal(ids, []string{"a", "b"}) {
		t.Fatalf("preferences: %v %v", ids, err)
	}
	if empty := svc.retryInstall(out, nil); len(empty.Targets) != 0 {
		t.Fatal("nil retry repeated activation")
	}
}

func TestInstallLibraryOnlyAndPreferenceFailure(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	source := t.TempDir()
	writeSkill(t, source, "only", validMD("only", "library"))
	found, origin, _, err := svc.Discover(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigPath(), []byte("skills: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ag := testAgent(p.Home, "a", "a/skills")
	out := svc.installWithTargets(found, origin, []agent.Agent{ag}, nil)
	if installTarget(t, out.result, "only / install").Outcome != core.Succeeded || installTarget(t, out.result, "only / activation").Outcome != core.Skipped || installTarget(t, out.result, installSettingsTarget).Outcome != core.Failed {
		t.Fatalf("result: %+v", out.result)
	}
	if scanOne(t, svc, []agent.Agent{ag}, "only").EnabledCount() != 0 {
		t.Fatal("library only activated a skill")
	}
	data, _ := os.ReadFile(p.ConfigPath())
	if string(data) != "skills: [\n" {
		t.Fatal("overwrote malformed config")
	}
	if err := os.WriteFile(p.ConfigPath(), []byte("# keep\ncustom:\n  value: true\nskills:\n  future: keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := svc.retryInstall(out, []string{installSettingsTarget})
	if r.AsError() != nil || len(r.Targets) != 1 {
		t.Fatalf("preference retry: %+v", r)
	}
	data, _ = os.ReadFile(p.ConfigPath())
	for _, want := range []string{"# keep", "future: keep", "value: true", "installTargets: []"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("lost config %q: %s", want, data)
		}
	}
	if err := svc.saveInstallTargets([]string{"a"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p.ConfigPath())
	hookSource := t.TempDir()
	if err := os.Mkdir(filepath.Join(hookSource, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookSource, "hooks", "hooks.json"), []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo fixture"}]}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	found, origin, _, err = svc.Discover(hookSource)
	if err != nil {
		t.Fatal(err)
	}
	hookOut := svc.installWithTargets(found, origin, []agent.Agent{ag}, nil)
	after, _ := os.ReadFile(p.ConfigPath())
	if hookOut.result.AsError() != nil || len(hookOut.result.Targets) != 1 || !strings.Contains(hookOut.result.Targets[0].State, "not enabled") || string(before) != string(after) {
		t.Fatalf("hooks-only import changed destinations or activation: %+v", hookOut)
	}
}

func TestInstallPickerReviewsSingleSkillAndCleansCancelledSource(t *testing.T) {
	p := testPaths(t)
	m := newTab(New(p))
	source := t.TempDir()
	writeSkill(t, source, "only", validMD("only", "source"))
	m.width, m.height = 80, 24
	m.agents = []agent.Agent{testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills")}
	msg := m.discoverCmd(source)().(discoverMsg)
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.Mkdir(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	msg.cleanup = staging
	m, cmd := m.update(msg)
	if cmd != nil || m.mode != skModePick || len(m.picker.ids) != 0 {
		t.Fatal("single-skill source bypassed destination review")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.ckind != confirmKindInstall || !strings.Contains(m.confirm.Question, "- skill: only") || !strings.Contains(m.confirm.Question, "Activate newly installed skills in: a") {
		t.Fatalf("review: %s", m.confirm.Question)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // defaults to No
	if m.mode != skModePick {
		t.Fatal("cancel did not return to destinations")
	}
	if _, err := os.Stat(p.LibraryDir()); !os.IsNotExist(err) {
		t.Fatal("review wrote library files")
	}
	if _, err := os.Stat(p.ConfigPath()); !os.IsNotExist(err) {
		t.Fatal("cancel saved preferences")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, cmd = m.update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.mode != skModeInstallBusy || !m.Capturing() {
		t.Fatal("install not locked while running")
	}
	if duplicate := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); duplicate != nil {
		t.Fatal("duplicate install accepted")
	}
	for _, c := range cmd().(tea.BatchMsg) {
		if msg, ok := c().(installFlowDoneMsg); ok {
			m, _ = m.update(msg)
		}
	}
	if m.report == nil || m.report.Result.AsError() != nil || !scanOne(t, m.svc, m.agents, "only").States["a"].On || scanOne(t, m.svc, m.agents, "only").States["b"].On {
		t.Fatalf("install completion: %+v", m.report)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatal("completed install leaked staging")
	}
	msg = m.discoverCmd(source)().(discoverMsg)
	msg.targets = append(msg.targets, "offline", "a")
	m.report = nil
	m, _ = m.update(msg)
	if !slices.Equal(m.picker.ids, []string{"a"}) || !strings.Contains(strings.Join(m.picker.origin.Notes, "\n"), "offline") {
		t.Fatal("saved targets silently expanded or unavailable destination hidden")
	}
	cleanup := filepath.Join(t.TempDir(), "staging")
	if err := os.Mkdir(cleanup, 0o755); err != nil {
		t.Fatal(err)
	}
	m.picker.cleanup = cleanup
	m, cmd = m.updatePick(tea.KeyPressMsg{Code: tea.KeyEscape})
	cmd()
	if _, err := os.Stat(cleanup); !os.IsNotExist(err) {
		t.Fatal("cancel leaked staged source")
	}
}

func TestInstallDestinationPickerAndHookSelection(t *testing.T) {
	found := []Found{{Name: "one", Valid: true}, {Name: "two", Valid: true}, {Name: "hook", Valid: true, Hook: &hooks.Found{}}}
	for _, size := range [][2]int{{36, 11}, {60, 19}, {120, 30}} {
		p := newPicker(found, Origin{}, "")
		p.agents = []agent.Agent{testAgent(t.TempDir(), "a", "a/skills"), testAgent(t.TempDir(), "b", "b/skills")}
		for range 2 {
			p = p.update(tea.KeyPressMsg{Code: 'a', Text: "a"}, size[0], size[1])
		}
		if p.sel[2] {
			t.Fatal("skills all/none opted a hook in")
		}
		p.destinations = true
		for i := range 3 {
			p.targetCursor = i
			v := p.view(size[0], size[1])
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] || !strings.Contains(ansi.Strip(v), "enter review") {
				t.Fatalf("destination picker %v: %s", size, v)
			}
		}
		m := Tab{width: size[0], height: size[1], mode: skModePick, picker: p}
		start, _ := p.destinationWindow(size[0], size[1])
		m, _ = m.click(tea.MouseClickMsg{X: 3, Y: 3, Button: tea.MouseLeft})
		if m.picker.targetCursor != start {
			t.Fatal("destination click missed the visible row")
		}
		p = p.updateDestinations(tea.KeyPressMsg{Code: 'a', Text: "a"})
		if len(p.ids) != 2 {
			t.Fatal("explicit all did not choose available agents")
		}
		p = p.updateDestinations(tea.KeyPressMsg{Code: '0', Text: "0"})
		if len(p.ids) != 0 {
			t.Fatal("library only retained destinations")
		}
	}
	// Escape returns from registry input/results instead of entering a busy mode.
	m := newTab(New(testPaths(t)))
	for _, mode := range []skMode{skModeRegistry, skModeRegistryPick} {
		m.mode = mode
		m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.mode != skModeList {
			t.Fatal("registry cancel got stuck")
		}
	}
}
