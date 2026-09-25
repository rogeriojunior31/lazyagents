package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// run feeds a tea.Cmd result back to Update, chaining until done.
func run(t *testing.T, m *Tab, cmd tea.Cmd) {
	t.Helper()
	for i := 0; cmd != nil && i < 5; i++ {
		msg := cmd()
		cmd = m.Update(msg)
	}
}

// TestApplyFlow covers the tab's risky path: key → confirm → write to the
// agent's live file.
func TestApplyFlow(t *testing.T) {
	home := t.TempDir()
	claude := agent.NewClaude(home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil { // makes Detect see it installed
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{claude}, core.PathsIn(home))
	if err := svc.Save(agent.ProviderProfile{Name: "cloud", BaseURL: "https://cloud/v1", Token: "secret"}); err != nil {
		t.Fatal(err)
	}

	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	if m.Count() != 1 || len(m.statuses) != 1 {
		t.Fatalf("load = %d profiles, %d agents", m.Count(), len(m.statuses))
	}
	// The tab applies by name only: the token never reaches the model.
	if p := m.profiles[0]; p.Token != "" || !p.HasToken {
		t.Errorf("profile in tab = %+v", p)
	}

	// 1 arms the confirm; nothing is written before "yes".
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if !m.Capturing() {
		t.Fatal("key 1 should open the confirm")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("the confirm was still open and the file was already written")
	}
	if view := m.View(); !strings.Contains(view, "cloud") || !strings.Contains(view, "~/.claude/settings.json") {
		t.Errorf("the confirm does not say what will change:\n%s", view)
	}

	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.Capturing() {
		t.Error("confirm stayed open after yes")
	}
	data, err := os.ReadFile(claude.ProviderFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://cloud/v1") {
		t.Errorf("profile was not applied:\n%s", data)
	}
	if m.statuses[0].Profile != "cloud" {
		t.Errorf("the tab did not reload state: %+v", m.statuses[0])
	}
	if view := m.View(); !strings.Contains(view, "●") || strings.Contains(view, "secret") {
		t.Errorf("wrong view (or with token):\n%s", view)
	}

	// 1 again on the agent that already has the profile = clear.
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.statuses[0].Active {
		t.Errorf("second 1 should clear: %+v", m.statuses[0])
	}
}

func TestCancelDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	claude := agent.NewClaude(home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{claude}, core.PathsIn(home))
	if err := svc.Save(agent.ProviderProfile{Name: "cloud", BaseURL: "https://cloud/v1"}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))

	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd != nil {
		t.Error("esc should not return a command")
	}
	if m.Capturing() {
		t.Error("esc should close the confirm")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Error("esc wrote to the agent's file")
	}
}

// typeText types s into the focused form field.
func typeText(m *Tab, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// The form creates the profile; when editing, an empty token keeps the saved
// one and the value never shows on screen.
func TestProfileForm(t *testing.T) {
	home := t.TempDir()
	svc := New(nil, core.PathsIn(home))
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if !m.Capturing() {
		t.Fatal("n should open the form")
	}
	typeText(&m, "cloud")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	typeText(&m, "ftp://x") // invalid: the form stays open with the error
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.form == nil || !strings.Contains(m.View(), "http") {
		t.Fatalf("invalid URL should keep the form open with the error:\n%s", m.View())
	}
	for range 7 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typeText(&m, "https://cloud/v1")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	typeText(&m, "secret")
	if strings.Contains(m.View(), "secret") {
		t.Fatal("typed token is shown in clear")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.form != nil {
		t.Fatalf("form did not close: %s", m.form.err)
	}
	p, err := svc.Profile("cloud")
	if err != nil || p.BaseURL != "https://cloud/v1" || p.Token != "secret" {
		t.Fatalf("saved profile = %+v, %v", p, err)
	}

	// Edit: change the model and rename, without typing a token.
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	typeText(&m, "2")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	typeText(&m, "big")
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if _, err := svc.Profile("cloud"); err == nil {
		t.Error("renaming should drop the old name")
	}
	p, err = svc.Profile("cloud2")
	if err != nil || p.Model != "big" || p.Token != "secret" {
		t.Fatalf("edit = %+v, %v", p, err)
	}
	for _, prof := range m.profiles {
		if prof.Token != "" {
			t.Errorf("token reached the tab: %+v", prof)
		}
	}
}

// Clicking the list selects that row's profile.
func TestClickSelectsProfile(t *testing.T) {
	svc := New(nil, core.PathsIn(t.TempDir()))
	for _, n := range []string{"a", "b"} {
		if err := svc.Save(agent.ProviderProfile{Name: n, Model: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Init())
	m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft}) // 2nd item
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m.Update(tea.MouseClickMsg{X: 80, Y: 2, Button: tea.MouseLeft}) // detail: ignored
	if m.cursor != 1 {
		t.Errorf("click on the detail moved the cursor: %d", m.cursor)
	}
}

func TestProfileFormSmallTerminal(t *testing.T) {
	for _, height := range []int{8, 11, 19} {
		for i := range fieldCount {
			form := newProfileForm(agent.ProviderProfile{}, false)
			form.setFocus(i)
			form.inputs[i].SetValue(strings.Repeat("x", 70) + "END")
			form.inputs[i].CursorEnd()
			view := form.view(36, height)
			plain := ansi.Strip(view)
			if lipgloss.Width(view) > 36 || lipgloss.Height(view) > height || !strings.Contains(plain, fieldLabels[i]) || !strings.Contains(plain, "save") || !strings.Contains(plain, "esc") {
				t.Fatalf("field %d at height %d unreachable:\n%s", i, height, plain)
			}
			if i != fToken && !strings.Contains(plain, "END") {
				t.Fatalf("cursor cut off:\n%s", plain)
			}
			if i == fToken && strings.Contains(plain, "END") {
				t.Fatal("token exposed")
			}
		}
	}
}

func TestProviderDetailAndModalMouse(t *testing.T) {
	m := newTab(New(nil, core.PathsIn(t.TempDir())))
	m.profiles = []agent.ProviderProfile{{Name: "first", BaseURL: "https://example.com/" + strings.Repeat("path/", 60)}, {Name: "second"}}
	m.statuses = []Status{{AgentID: "codex", AgentName: "Codex", File: "/tmp/LAST-FILE"}}
	for _, w := range []int{36, 76, 116} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 11})
		for range 60 {
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		view := m.View()
		if !strings.Contains(ansi.Strip(view), "LAST-FILE") || lipgloss.Width(view) > w || lipgloss.Height(view) > 11 {
			t.Fatalf("detail unreachable:\n%s", ansi.Strip(view))
		}
	}
	if m.cursor != 0 {
		t.Fatal("shift+↓ changed the profile")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 || m.detailOff != 0 {
		t.Fatal("selection did not reset the detail")
	}
	m.ask(strings.Repeat("long question\n", 40), func() tea.Msg { t.Fatal("should not apply"); return nil })
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	if m.cursor != 1 {
		t.Fatal("mouse changed the selection behind the dialog")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.confirm != nil {
		t.Fatal("esc did not cancel")
	}
}
