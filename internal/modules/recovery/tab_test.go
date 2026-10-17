package recovery

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestTabPreviewCancelAndRestore(t *testing.T) {
	svc, _, path, id := fixture(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	cmd := m.Update(events.TabActivated{ID: m.ID()})
	m.Update(cmd())
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	if m.preview == nil || m.preview.ID != id || strings.Contains(m.View(), "provider-secret") {
		t.Fatal("missing redacted preview")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.preview != nil {
		t.Fatal("escape did not cancel preview")
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
		t.Fatal("preview wrote the live file")
	}
	cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil {
		t.Fatal("restore skipped explicit confirmation")
	}
	cmd = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	result := cmd().(restoredMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	batch := m.Update(result)().(tea.BatchMsg)
	changed := false
	for _, command := range batch {
		msg := command()
		if event, ok := msg.(events.AgentConfigChanged); ok && event.AgentID == "claude-code" {
			changed = true
		}
		m.Update(msg)
	}
	if !changed {
		t.Fatal("other tabs were not told to reload config state")
	}
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "original-private") {
		t.Fatal("restore did not recover the selected backup")
	}
}

func TestRecoveryTabSearchAndSmallTerminals(t *testing.T) {
	svc, _, _, _ := fixture(t)
	m := newTab(svc)
	m.Update(m.loadCmd()())
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.PasteMsg{Content: "claude-code provider.apply"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.matches) != 1 {
		t.Fatal("agent/operation search failed")
	}
	for _, width := range []int{30, 40, 100} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		view := m.View()
		if lipgloss.Width(view) > width || lipgloss.Height(view) > 11 {
			t.Fatalf("list does not fit at %d: %s", width, view)
		}
	}
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	for _, width := range []int{30, 40, 100} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		view := m.View()
		if lipgloss.Width(view) > width || lipgloss.Height(view) > 11 || !strings.Contains(view, "esc") {
			t.Fatalf("preview does not fit at %d: %s", width, view)
		}
	}
}

func TestRecoveryReloadsOnReentry(t *testing.T) {
	svc, c, _, _ := fixture(t)
	m := newTab(svc)
	m.Update(m.Update(events.TabActivated{ID: m.ID()})())
	before := len(m.entries)
	if err := c.AddHook(agent.Hook{Event: agent.HookStop, Command: "echo new backup"}, svc.dir); err != nil {
		t.Fatal(err)
	}
	cmd := m.Update(events.TabActivated{ID: m.ID()})
	if cmd == nil {
		t.Fatal("returning to Recovery retained the stale backup inventory")
	}
	m.Update(cmd())
	if len(m.entries) != before+1 {
		t.Fatal("new snapshot not visible on reentry")
	}
}
