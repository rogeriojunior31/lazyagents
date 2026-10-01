package agents

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func detected() events.AgentsDetected {
	return events.AgentsDetected{Agents: []agent.Agent{
		{ID: "b", Name: "Second"},
		{ID: "a", Name: "First", Installed: true, Version: "1.2", ManagedDir: "/tmp/skills",
			Detail: strings.Repeat("long detail ", 50) + "END_WARNING"},
	}}
}

func TestAgentDetailAccessible(t *testing.T) {
	for _, w := range []int{36, 76, 116, 160} {
		m := newTab("/tmp", nil)
		m.Update(tea.WindowSizeMsg{Width: w, Height: 11})
		m.Update(detected())
		// wheel over the detail scrolls the text, not the selection
		over := tea.MouseWheelMsg{Button: tea.MouseWheelDown, Y: m.split().ListH + 1}
		if sp := m.split(); sp.Side { // wide: the detail is on the right
			over = tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: sp.ListW + 3, Y: 2}
		}
		if (w >= 160) != m.split().Side {
			t.Fatalf("width %d: detail beside = %v", w, m.split().Side)
		}
		m.Update(over)
		if w >= 160 && !strings.Contains(ansi.Strip(m.View()), "provider") {
			t.Fatalf("width %d: capability columns missing beside the detail", w)
		}
		if m.detailOff == 0 || m.cursor != 0 {
			t.Fatal("wheel did not scroll only the detail")
		}
		for range 40 {
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		view := m.View()
		if !strings.Contains(ansi.Strip(view), "END_WARNING") || lipgloss.Width(view) > w || lipgloss.Height(view) > 11 {
			t.Fatalf("detail unreachable at %d:\n%s", w, ansi.Strip(view))
		}
		if m.cursor != 0 {
			t.Fatal("shift+↓ changed the agent")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if m.cursor != 1 || m.detailOff != 0 {
			t.Fatal("new selection did not reset to the top")
		}
	}
}

func TestAgentTableAtAGlance(t *testing.T) {
	m := newTab("/tmp", nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(detected())
	m.Update(events.SessionsLoaded{Sessions: []agent.Session{{AgentID: "a"}}})
	m.Update(events.SkillsScanned{ActiveByAgent: map[string]int{"a": 1}})
	plain := ansi.Strip(m.View())
	for _, want := range []string{"1 of 2 installed", "agent", "sessions", "provider", "● First", "1.2", "○ Second", "missing"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "1 sessions") || strings.Contains(plain, "1 active skills") {
		t.Error("wrong plural")
	}
	if strings.Index(plain, "First") > strings.Index(plain, "Second") {
		t.Error("installed agents should come before missing ones")
	}
	// On a narrow screen capabilities move from the table to the detail.
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	if plain = ansi.Strip(m.View()); strings.Contains(plain, "provider  usage") || !strings.Contains(plain, "manages") {
		t.Errorf("capabilities on a narrow screen:\n%s", plain)
	}
}

func TestAgentClickSelectsRow(t *testing.T) {
	m := newTab("/tmp", nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(detected())
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: tableTop + 1})
	if m.cursor != 1 {
		t.Fatalf("click selected %d", m.cursor)
	}
}
