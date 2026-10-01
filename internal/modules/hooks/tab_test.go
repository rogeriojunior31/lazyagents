package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// run feeds a tea.Cmd result back to Update until the chain ends.
func run(t *testing.T, m *Tab, cmd tea.Cmd) {
	t.Helper()
	for i := 0; cmd != nil && i < 5; i++ {
		cmd = m.Update(cmd())
	}
}

// TestInstallFlow covers the tab's risky path: key → confirm → write to the
// agent's live file.
func TestInstallFlow(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	if err := svc.Save(Hook{Name: "doctor", Description: "runs doctor",
		Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "lazyagents doctor"}}}); err != nil {
		t.Fatal(err)
	}

	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	if m.Count() != 1 || len(m.statuses) != 1 {
		t.Fatalf("load = %d hooks, %d agents", m.Count(), len(m.statuses))
	}

	// space arms the confirm; nothing is written before "yes".
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !m.Capturing() {
		t.Fatal("space should open the confirm")
	}
	if _, err := os.Stat(claude.HooksFile()); !os.IsNotExist(err) {
		t.Fatal("wrote while the confirm was open")
	}
	if view := m.View(); !strings.Contains(view, "lazyagents doctor") || !strings.Contains(view, filepath.Join("~", ".claude", "settings.json")) {
		t.Errorf("the confirm does not say what changes:\n%s", view)
	}

	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	data, err := os.ReadFile(claude.HooksFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "lazyagents doctor") {
		t.Errorf("hook not installed:\n%s", data)
	}
	if len(m.statuses[0].Enabled) != 1 {
		t.Errorf("the tab did not reload: %+v", m.statuses[0])
	}
	if view := m.View(); !strings.Contains(view, "●") {
		t.Errorf("matrix without marker:\n%s", view)
	}

	// space again removes it.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if len(m.statuses[0].Enabled) != 0 {
		t.Errorf("second space should remove: %+v", m.statuses[0])
	}
}

func TestEscCancelsWrite(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	if err := svc.Save(Hook{Name: "x", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo x"}}}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Capturing() {
		t.Error("esc should close the confirm")
	}
	if _, err := os.Stat(claude.HooksFile()); !os.IsNotExist(err) {
		t.Error("esc wrote to the agent's file")
	}
}

// The matrix marks with – an agent that does not fire the hook's event.
func TestMatrixMarksUnsupportedEvent(t *testing.T) {
	svc, home := testService(t)
	svc.adapters = append(svc.adapters, agent.NewCodex(home))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(Hook{Name: "stop", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo x"}}}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	if !strings.Contains(m.View(), "–") {
		t.Errorf("missing the unsupported-event marker:\n%s", m.View())
	}
}

// enter opens the command list; space turns off the command under the cursor,
// with a confirm when the package is installed.
func TestCommandModeToggle(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	a := agent.Hook{Event: agent.HookSessionStart, Command: "echo a"}
	b := agent.Hook{Event: agent.HookStop, Command: "echo b"}
	if err := svc.Save(Hook{Name: "pack", Hooks: []agent.Hook{a, b}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enable("pack", "claude-code"); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.cmdMode || !m.Capturing() {
		t.Fatal("enter should open the command list")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if !strings.Contains(m.View(), cmdCursorMark) {
		t.Errorf("no cursor on the command:\n%s", m.View())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if m.confirm == nil {
		t.Fatal("turning off an installed command should ask for confirmation")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	installed, _ := claude.ReadHooks()
	if !containsHook(installed, a) || containsHook(installed, b) {
		t.Errorf("space should have removed only b: %+v", installed)
	}
	if h, _ := svc.Get("pack"); !h.IsOff(1) {
		t.Errorf("library did not record it: %+v", h)
	}
	if !m.cmdMode {
		t.Error("command mode should stay after the toggle")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.cmdMode {
		t.Error("esc should go back to the list")
	}
}

func TestEmptyHooksShowsNextStep(t *testing.T) {
	svc, _ := testService(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Skills") || !strings.Contains(view, "press i") || strings.Contains(view, "agent N") || !strings.Contains(view, "help") {
		t.Fatalf("empty state without guidance:\n%s", view)
	}
}

func TestDetailScrollAndCommandFocus(t *testing.T) {
	svc, _ := testService(t)
	for _, width := range []int{36, 100} {
		m := newTab(svc)
		m.lib = []Hook{{Name: "pack", Description: strings.Repeat("long description ", 40),
			Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo first"}, {Event: agent.HookStop, Command: "echo FINAL"}}}, {Name: "other"}}
		m.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		for i := 0; i < 100; i++ {
			m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		}
		if view := ansi.Strip(m.View()); !strings.Contains(view, "FINAL") {
			t.Fatalf("width %d: end of detail unreachable:\n%s", width, view)
		}
		m.detailOff = 0
		x, y := m.listWidth()+2, 2
		if !m.split().Side {
			x, y = 2, m.listHeight()+1
		}
		m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: x, Y: y})
		if m.cursor != 0 || m.detailOff != 1 {
			t.Fatalf("wheel on the detail moved the list: cursor=%d scroll=%d", m.cursor, m.detailOff)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 2, Y: 2})
		if m.cursor != 0 || m.cmdCursor != 1 {
			t.Fatal("wheel must move through commands without changing the hook")
		}
		m.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "COMMANDS · 2/2") || !strings.Contains(view, cmdCursorMark) || !strings.Contains(view, "FINAL") || strings.Contains(view, "LIBRARY") {
			t.Fatalf("selected command unreachable after resize:\n%s", view)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.cmdMode || !strings.Contains(ansi.Strip(m.View()), "LIBRARY") {
			t.Fatal("esc must restore the library")
		}
	}
}

func TestLongCommandsReadableWithoutChangingSelection(t *testing.T) {
	svc, _ := testService(t)
	for _, width := range []int{36, 100} {
		m := newTab(svc)
		command := "echo START " + strings.Repeat("argument ", 50) + "END_COMMAND"
		m.lib = []Hook{{Name: "pack", Off: []int{0}, Hooks: []agent.Hook{
			{Event: agent.HookStop, Command: command, Matcher: "Write|Edit|Bash|END_MATCHER", Async: true, Timeout: 123},
			{Event: agent.HookStop, Command: "echo next"},
		}}}
		m.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		var seen strings.Builder
		for i := 0; i < 100; i++ {
			view := ansi.Strip(m.View())
			seen.WriteString(view + "\n")
			if !strings.Contains(view, cmdCursorMark+" [ ]") {
				t.Fatalf("selection lost while reading: %s", view)
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("line wider than %d columns: %s", width, line)
				}
			}
			if cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown}); cmd != nil {
				t.Fatal("reading a command must not trigger an action")
			}
		}
		for _, want := range []string{"START", "END_COMMAND", "END_MATCHER", "disabled", "async", "123s"} {
			if !strings.Contains(seen.String(), want) {
				t.Fatalf("width %d: %q unreachable by paging", width, want)
			}
		}
		if m.cmdCursor != 0 || !m.lib[0].IsOff(0) || m.confirm != nil {
			t.Fatal("reading changed the selection or state")
		}
		for i := 0; i < 100; i++ {
			m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		}
		if m.detailOff != 0 {
			t.Fatal("pgup did not return to the top")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if m.cmdCursor != 1 || !strings.Contains(ansi.Strip(m.View()), "next") {
			t.Fatal("navigation did not follow the next command")
		}
	}
}
