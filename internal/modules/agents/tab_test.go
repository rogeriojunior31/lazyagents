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

func TestAgentDetailAccessible(t *testing.T) {
	for _, w := range []int{36, 76, 116} {
		m := newTab("/tmp", nil)
		m.Update(tea.WindowSizeMsg{Width: w, Height: 11})
		m.Update(events.AgentsDetected{Agents: []agent.Agent{
			{ID: "a", Name: "Primeiro", Installed: true, ManagedDir: "/tmp/skills", SharedNote: strings.Repeat("aviso longo ", 50)},
			{ID: "b", Name: "Segundo"},
		}})
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		if m.detailOff == 0 || m.cursor != 0 {
			t.Fatal("mouse não rolou só o detalhe")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		view := m.View()
		if !strings.Contains(ansi.Strip(view), "O LAZYAGENTS GERENCIA") || lipgloss.Width(view) > w || lipgloss.Height(view) > 11 {
			t.Fatalf("detalhe inacessível:\n%s", ansi.Strip(view))
		}
		m.Update(tea.WindowSizeMsg{Width: 116, Height: 100})
		if !strings.Contains(ansi.Strip(m.View()), "Primeiro") {
			t.Fatal("resize perdeu início")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if m.cursor != 1 || m.detailOff != 0 {
			t.Fatal("nova seleção não voltou ao topo")
		}
	}
}
