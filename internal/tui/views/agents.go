package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
)

// Agents é a aba de diagnóstico: o que está instalado, versão, onde ficam as
// skills de cada agente e quantas sessões ele tem.
type Agents struct {
	agents        []agent.Agent
	counts        map[string]int // sessões por agente
	width, height int
}

func NewAgents() Agents { return Agents{counts: map[string]int{}} }

func (m Agents) Init() tea.Cmd { return nil }

func (m Agents) Capturing() bool { return false }

func (m Agents) Update(msg tea.Msg) (Agents, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case AgentsMsg:
		m.agents = msg.Agents
	case sessionsMsg:
		counts := map[string]int{}
		for _, s := range msg.sessions {
			counts[s.AgentID]++
		}
		m.counts = counts
	}
	return m, nil
}

func (m Agents) View() string {
	if len(m.agents) == 0 {
		return stHint.Render("detectando agentes…")
	}
	var b strings.Builder
	for _, ag := range m.agents {
		mark := stErr.Render("✗")
		if ag.Installed {
			mark = stOn.Render("✓")
		}
		head := fmt.Sprintf("%s %s", mark, stTitle.Render(ag.Name))
		if ag.Version != "" {
			head += stHint.Render("  " + ag.Version)
		}
		b.WriteString(head + "\n")
		b.WriteString(stHint.Render("   "+ag.Detail) + "\n")
		if ag.Installed {
			if ag.SupportsSkills() {
				b.WriteString(stText.Render("   skills: "+ag.ManagedDir) + "\n")
				if len(ag.ReadDirs) > 1 {
					b.WriteString(stHint.Render("   também lê: "+strings.Join(ag.ReadDirs[1:], ", ")) + "\n")
				}
				if ag.SharedNote != "" {
					b.WriteString(stLocal.Render("   ⚠ "+ag.SharedNote) + "\n")
				}
			}
			if n := m.counts[ag.ID]; n > 0 {
				b.WriteString(stText.Render(fmt.Sprintf("   sessões: %d", n)) + "\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(stHint.Render("legenda da matriz de skills: ") +
		stOn.Render("letra") + stHint.Render(" ativa (gerenciada) · ") +
		stShared.Render("letra") + stHint.Render(" via dir compartilhado · ") +
		stLocal.Render("letra") + stHint.Render(" local · ") +
		stOff.Render("·") + stHint.Render(" inativa"))
	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}
