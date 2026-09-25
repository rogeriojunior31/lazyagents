package sessions

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"strings"
	"testing"
)

type deleteRecorder struct {
	fakeAdapter
	deleted []string
}

func (a *deleteRecorder) DeleteSession(s agent.Session, _ string) error {
	a.deleted = append(a.deleted, s.ID)
	return nil
}

func TestDeleteDialogDefaultsToNoAndKeepsTargets(t *testing.T) {
	home := t.TempDir()
	adapter := &deleteRecorder{fakeAdapter: fakeAdapter{id: "a"}}
	m := newTab(New([]agent.Adapter{adapter}, core.PathsIn(home)), home)
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
	first := agent.Session{ID: "first", AgentID: "a", AgentName: "Agent", Title: "First chat"}
	second := agent.Session{ID: "second", AgentID: "a", Title: "Second chat"}
	m.Update(events.SessionsLoaded{Sessions: []agent.Session{first, second}})
	for _, cancel := range []rune{tea.KeyEnter, tea.KeyEscape} {
		m.Update(tea.KeyPressMsg{Code: 'd'})
		view := m.View()
		if !m.Capturing() || !strings.Contains(ansi.Strip(view), "Yes") || lipgloss.Height(view) > 11 {
			t.Fatal("dialog unreachable")
		}
		if cmd := m.Update(tea.KeyPressMsg{Code: cancel}); cmd != nil || m.confirm != nil {
			t.Fatal("cancel returned an operation")
		}
	}
	m.Update(tea.KeyPressMsg{Code: 'd'})
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 5})
	if len(m.deleteTargets) != 1 || m.deleteTargets[0].ID != "first" {
		t.Fatal("mouse changed the targets")
	}
	m.Update(events.SessionsLoaded{Sessions: []agent.Session{second}})
	cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	if cmd == nil {
		t.Fatal("confirm produced no operation")
	}
	cmd()
	if len(adapter.deleted) != 1 || adapter.deleted[0] != "first" {
		t.Fatalf("targets changed: %v", adapter.deleted)
	}
}

func TestSessionFiltersKeepHints(t *testing.T) {
	home := t.TempDir()
	m := newTab(New(nil, core.PathsIn(home)), home)
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
	m.agentFilter = "codex"
	m.searchIDs = map[string]bool{}
	m.searchQuery = "text"
	m.applyItems()
	view := m.View()
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "agent:") || !strings.Contains(plain, "search:") || !strings.Contains(plain, "help") || lipgloss.Height(view) > 11 {
		t.Fatalf("filters hid the actions:\n%s", plain)
	}
}
