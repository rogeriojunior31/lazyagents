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
		{ID: "b", Name: "Segundo"},
		{ID: "a", Name: "Primeiro", Installed: true, Version: "1.2", ManagedDir: "/tmp/skills",
			SharedNote: strings.Repeat("aviso longo ", 50) + "FIM_AVISO"},
	}}
}

func TestAgentDetailAccessible(t *testing.T) {
	for _, w := range []int{36, 76, 116} {
		m := newTab("/tmp", nil)
		m.Update(tea.WindowSizeMsg{Width: w, Height: 11})
		m.Update(detected())
		// roda sobre a faixa de detalhe rola o texto, não troca de agente
		m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, Y: m.split().ListH + 1})
		if m.detailOff == 0 || m.cursor != 0 {
			t.Fatal("mouse não rolou só o detalhe")
		}
		for range 40 {
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		view := m.View()
		if !strings.Contains(ansi.Strip(view), "FIM_AVISO") || lipgloss.Width(view) > w || lipgloss.Height(view) > 11 {
			t.Fatalf("detalhe inacessível em %d:\n%s", w, ansi.Strip(view))
		}
		if m.cursor != 0 {
			t.Fatal("shift+↓ trocou de agente")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if m.cursor != 1 || m.detailOff != 0 {
			t.Fatal("nova seleção não voltou ao topo")
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
	for _, want := range []string{"1 of 2 installed", "agent", "sessions", "provider", "● Primeiro", "1.2", "○ Segundo", "missing"} {
		if !strings.Contains(plain, want) {
			t.Errorf("falta %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "1 sessions") || strings.Contains(plain, "1 active skills") {
		t.Error("plural errado")
	}
	if strings.Index(plain, "Primeiro") > strings.Index(plain, "Segundo") {
		t.Error("instalados deveriam vir antes dos ausentes")
	}
	// Em tela estreita as capacidades saem da tabela e vão para o detalhe.
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	if plain = ansi.Strip(m.View()); strings.Contains(plain, "provider  usage") || !strings.Contains(plain, "manages") {
		t.Errorf("capacidades em tela estreita:\n%s", plain)
	}
}

func TestAgentClickSelectsRow(t *testing.T) {
	m := newTab("/tmp", nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(detected())
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: tableTop + 1})
	if m.cursor != 1 {
		t.Fatalf("clique selecionou %d", m.cursor)
	}
}
