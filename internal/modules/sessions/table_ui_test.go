package sessions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// tableTab builds the tab with n sessions of agent "a" across three projects.
func tableTab(t *testing.T, n, w, h int) *Tab {
	t.Helper()
	home := t.TempDir()
	tab := newTab(New([]agent.Adapter{fakeAdapter{id: "a"}}, core.PathsIn(home)), home)
	m := &tab
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	var ss []agent.Session
	for i := range n {
		ss = append(ss, agent.Session{ID: fmt.Sprintf("s%02d", i), AgentID: "a", AgentName: "Agent",
			Title: fmt.Sprintf("session-%02d %s", i, strings.Repeat("text ", 15)), CWD: fmt.Sprintf("/p/proj%d", i%3),
			MTime: time.Now().Add(-time.Duration(i) * time.Hour)})
	}
	m.Update(events.SessionsLoaded{Sessions: ss})
	return m
}

func TestSessionTableOneLinePerConversation(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 20}, {130, 30}} {
		w, h := size[0], size[1]
		m := tableTab(t, 30, w, h)
		for i := range 30 {
			view := m.View()
			plain := ansi.Strip(view)
			if lipgloss.Width(view) > w || lipgloss.Height(view) > h {
				t.Fatalf("%v: view %dx%d outside the area", size, lipgloss.Width(view), lipgloss.Height(view))
			}
			if !strings.Contains(plain, fmt.Sprintf("session-%02d", i)) || !strings.Contains(plain, "help") {
				t.Fatalf("%v: session %d or help off screen:\n%s", size, i, plain)
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
	}
	// With room, far more sessions fit than the ~4 of the old 3-line layout.
	m := tableTab(t, 30, 130, 30)
	if got := strings.Count(ansi.Strip(m.View()), "session-"); got < 20 {
		t.Errorf("only %d sessions visible at 130×30", got)
	}
}

func TestSessionDetailShowsResumeFirst(t *testing.T) {
	m := tableTab(t, 3, 80, 20)
	plain := ansi.Strip(m.View())
	resume, agentLine := strings.Index(plain, "a resume s00"), strings.Index(plain, "agent   Agent")
	if resume < 0 {
		t.Fatalf("resume command outside the detail:\n%s", plain)
	}
	if agentLine >= 0 && agentLine < resume {
		t.Error("metadata before the resume command")
	}
	// The session folder (/p/proj0) does not exist: resume uses /dir/a and the detail warns.
	for range 10 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	}
	if !strings.Contains(ansi.Strip(m.View()), "no longer exists") && !strings.Contains(m.detailVP.View(), "no longer exists") {
		t.Error("no missing-folder warning")
	}
	if m.list.Index() != 0 {
		t.Error("shift+↓ changed the selected session")
	}
}

func TestSessionGroupHeadersAndBatchSelect(t *testing.T) {
	m := tableTab(t, 6, 100, 24)
	m.Update(tea.KeyPressMsg{Code: 'g'})
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "▸ proj0 · a (2)") {
		t.Fatalf("group header missing:\n%s", plain)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // on the header: selects the group
	if n := len(m.selectedSessions()); n != 2 {
		t.Fatalf("group selected %d sessions", n)
	}
	if plain = ansi.Strip(m.View()); !strings.Contains(plain, "2 selected") || strings.Count(plain, "✓") != 2 {
		t.Errorf("batch selection not shown:\n%s", plain)
	}
}

func TestSessionMouseByRegion(t *testing.T) {
	m := tableTab(t, 10, 80, 24)
	sp := m.split()
	_, _, top := m.tableWindow(sp.ListW, sp.ListH)
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: top + 2})
	if m.list.Index() != 2 {
		t.Fatalf("click selected %d", m.list.Index())
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: sp.ListH + 1})
	if m.list.Index() != 2 {
		t.Error("wheel over the detail changed the session")
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: top})
	if m.list.Index() != 3 {
		t.Error("wheel over the table did not scroll")
	}
}

// A cost the agent recorded itself (Crush, Pi) is shown as exact, and a
// session with a cost but no tokens (Crush) shows no token line.
func TestSessionDetailRecordedCost(t *testing.T) {
	m := tableTab(t, 3, 100, 24)
	m.Update(usageMsg{id: "s00", ok: true, usage: agent.Usage{Cost: 1.5, CostKnown: true}, cost: 1.5, hasCost: true})
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "$1.50") || strings.Contains(plain, "~$1.50") || strings.Contains(plain, "tokens") {
		t.Errorf("recorded cost detail:\n%s", plain)
	}
	m.Update(usageMsg{id: "s00", ok: true, usage: agent.Usage{Input: 1000, Model: "claude-opus-4-8"}, cost: 0.01, hasCost: true})
	if plain = ansi.Strip(m.View()); !strings.Contains(plain, "~$0.01") || !strings.Contains(plain, "tokens") {
		t.Errorf("estimated cost detail:\n%s", plain)
	}
}
