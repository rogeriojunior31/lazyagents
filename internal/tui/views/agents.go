package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Agents é a aba de diagnóstico: cards com o que está instalado, versão, onde
// ficam as skills de cada agente e quantas sessões ele tem.
type Agents struct {
	agents        []agent.Agent
	counts        map[string]int // sessões por agente
	skillCounts   map[string]int // skills visíveis por agente
	width, height int
}

func NewAgents() Agents { return Agents{counts: map[string]int{}, skillCounts: map[string]int{}} }

func (m Agents) Init() tea.Cmd { return nil }

// InstalledCount conta os agentes detectados como instalados (contador da aba).
func (m Agents) InstalledCount() int {
	n := 0
	for _, ag := range m.agents {
		if ag.Installed {
			n++
		}
	}
	return n
}

func (m Agents) Capturing() bool { return false }

func (m Agents) step(msg tea.Msg) (Agents, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case events.AgentsDetected:
		m.agents = msg.Agents
	case events.SessionsLoaded:
		counts := map[string]int{}
		for _, s := range msg.Sessions {
			counts[s.AgentID]++
		}
		m.counts = counts
	case events.SkillsScanned:
		counts := map[string]int{}
		for _, sk := range msg.Skills {
			for id, st := range sk.States {
				if st.On {
					counts[id]++
				}
			}
		}
		m.skillCounts = counts
	}
	return m, nil
}

var (
	cardVer   = lipgloss.NewStyle().Foreground(theme.OK)
	cardLabel = lipgloss.NewStyle().Foreground(theme.Subtle)
	cardValue = lipgloss.NewStyle().Foreground(theme.Text)
)

// tilde encurta o home para ~ na exibição.
func tilde(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func (m Agents) card(ag agent.Agent, w int) string {
	p := components.Panel{Title: ag.Name, Focused: ag.Installed, Width: w}
	var b strings.Builder
	if ag.Installed {
		b.WriteString(stOn.Render("● instalado"))
		if ag.Version != "" {
			b.WriteString(cardLabel.Render("  ") + cardVer.Render(ag.Version))
		}
	} else {
		b.WriteString(stOff.Render("○ não instalado"))
	}
	b.WriteString("\n")

	home := ""
	if len(ag.ReadDirs) > 0 { // deduz o home do primeiro dir (~/...)
		if i := strings.Index(ag.ReadDirs[0], "/."); i > 0 {
			home = ag.ReadDirs[0][:i]
		}
	}
	if ag.Installed {
		if ag.SupportsSkills() {
			b.WriteString(cardLabel.Render("skills   ") + cardValue.Render(tilde(ag.ManagedDir, home)))
			if n := m.skillCounts[ag.ID]; n == 1 {
				b.WriteString(stOn.Render("  (1 ativa)"))
			} else if n > 1 {
				b.WriteString(stOn.Render(fmt.Sprintf("  (%d ativas)", n)))
			}
			b.WriteString("\n")
			if len(ag.ReadDirs) > 1 {
				var extras []string
				for _, d := range ag.ReadDirs[1:] {
					extras = append(extras, tilde(d, home))
				}
				b.WriteString(cardLabel.Render("também lê ") + cardLabel.Render(strings.Join(extras, " · ")) + "\n")
			}
		} else {
			b.WriteString(cardLabel.Render("skills   ") + stOff.Render("sem diretório local") + "\n")
		}
		b.WriteString(cardLabel.Render("sessões  "))
		if n := m.counts[ag.ID]; n > 0 {
			b.WriteString(cardValue.Render(fmt.Sprintf("%d", n)))
		} else {
			b.WriteString(stOff.Render("nenhuma"))
		}
		b.WriteString("\n")
		if ag.SharedNote != "" {
			b.WriteString(stLocal.Render("⚠ " + ag.SharedNote))
		}
	} else if ag.Detail != "não instalado" && ag.Detail != "" {
		b.WriteString(cardLabel.Render(ag.Detail))
	}

	inner := p.ContentWidth()
	return p.Render(lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n")))
}

func (m Agents) View() string {
	if len(m.agents) == 0 {
		return stHint.Render("detectando agentes…")
	}
	cardW := (m.width - 4) / 2
	if cardW < 30 {
		cardW = m.width - 2
	}
	var cards []string
	for _, ag := range m.agents {
		cards = append(cards, m.card(ag, cardW))
	}
	var rows []string
	if cardW == m.width-2 { // coluna única em telas estreitas
		rows = cards
	} else {
		for i := 0; i < len(cards); i += 2 {
			if i+1 < len(cards) {
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i], " ", cards[i+1]))
			} else {
				rows = append(rows, cards[i])
			}
		}
	}
	legend := stHint.Render("matriz de skills: ") +
		stOn.Render("●") + stHint.Render(" ativa gerenciada · ") +
		stShared.Render("◆") + stHint.Render(" via dir compartilhado · ") +
		stLocal.Render("▪") + stHint.Render(" local · ") +
		stOff.Render("○") + stHint.Render(" inativa")
	return lipgloss.JoinVertical(lipgloss.Left, append(rows, "", legend)...)
}

// --- module.Module ---

func (m *Agents) ID() string    { return "agents" }
func (m *Agents) Title() string { return "Agentes" }

// Update aplica a mensagem e guarda o novo estado (semântica de ponteiro do
// module.Module). events.Reload equivale à tecla r.
func (m *Agents) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(events.Reload); ok {
		msg = tea.KeyPressMsg{Code: 'r', Text: "r"}
	}
	nm, cmd := m.step(msg)
	*m = nm
	return cmd
}

// Count é o contador da aba: agentes instalados.
func (m *Agents) Count() int { return m.InstalledCount() }

// ClearToast: a aba Agentes não tem toast.
func (m *Agents) ClearToast() {}
