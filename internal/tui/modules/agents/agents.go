package agents

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Agents é a aba de diagnóstico: cards com o que está instalado, versão, onde
// ficam as skills de cada agente e quantas sessões ele tem.
type Agents struct {
	agents        []agent.Agent
	counts        map[string]int // sessões por agente
	skillCounts   map[string]int // skills visíveis por agente
	width, height int
	scroll        int
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
	case tea.KeyPressMsg:
		switch msg.String() {
		case "down", "j":
			m.scroll++
		case "up", "k":
			m.scroll = max(0, m.scroll-1)
		case "home", "g":
			m.scroll = 0
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			m.scroll += 3
		case tea.MouseWheelUp:
			m.scroll = max(0, m.scroll-3)
		}
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
	if m.width > 0 {
		m.scroll = min(m.scroll, max(0, lipgloss.Height(m.dashboard())-max(1, m.height-1)))
	}
	return m, nil
}

var (
	cardVer = lipgloss.NewStyle().Foreground(theme.OK)
)

// cardContent monta o miolo do card de um agente instalado, já quebrado na
// largura útil do Panel de largura w.
func (m Agents) cardContent(ag agent.Agent, w int) string {
	var b strings.Builder
	b.WriteString(kit.StOn.Render("● instalado"))
	if ag.Version != "" {
		b.WriteString("  " + cardVer.Render(ag.Version))
	}
	b.WriteString("\n\n")
	b.WriteString(kit.StTitle.Render(fmt.Sprintf("%d", m.skillCounts[ag.ID])) + kit.CardLabel.Render(" skills ativas    ") +
		kit.StTitle.Render(fmt.Sprintf("%d", m.counts[ag.ID])) + kit.CardLabel.Render(" sessões") + "\n\n")

	home := ""
	if len(ag.ReadDirs) > 0 { // deduz o home do primeiro dir (~/...)
		if i := strings.Index(ag.ReadDirs[0], "/."); i > 0 {
			home = ag.ReadDirs[0][:i]
		}
	}
	if ag.SupportsSkills() {
		b.WriteString(kit.CardLabel.Render("skills     ") + kit.CardValue.Render(core.Tilde(ag.ManagedDir, home)) + "\n")
		if len(ag.ReadDirs) > 1 {
			var extras []string
			for _, d := range ag.ReadDirs[1:] {
				extras = append(extras, core.Tilde(d, home))
			}
			b.WriteString(kit.CardLabel.Render("também lê  ") + kit.CardLabel.Render(strings.Join(extras, " · ")) + "\n")
		}
	} else {
		b.WriteString(kit.CardLabel.Render("skills     ") + kit.StOff.Render("sem diretório local") + "\n")
	}
	if ag.SharedNote != "" {
		b.WriteString("\n" + kit.StLocal.Render("⚠ "+ag.SharedNote))
	}

	inner := components.Panel{Width: w}.ContentWidth()
	return lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n"))
}

// cards renderiza os agentes instalados lado a lado; cada par divide a altura
// do mais alto, para as linhas do grid ficarem alinhadas.
func (m Agents) cards(installed []agent.Agent, w int, perRow int) []string {
	var rows []string
	for i := 0; i < len(installed); i += perRow {
		group := installed[i:min(len(installed), i+perRow)]
		contents := make([]string, len(group))
		h := 0
		for j, ag := range group {
			contents[j] = m.cardContent(ag, w)
			h = max(h, lipgloss.Height(contents[j]))
		}
		var row []string
		for j, ag := range group {
			if j > 0 {
				row = append(row, "  ")
			}
			p := components.Panel{Title: ag.Name, Width: w, Height: h + 2, Border: theme.AgentColor(ag.ID)}
			row = append(row, p.Render(contents[j]))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return rows
}

func (m Agents) View() string {
	lines := strings.Split(m.dashboard(), "\n")
	h := max(1, m.height-1)
	start := min(m.scroll, max(0, len(lines)-h))
	body := strings.Join(lines[start:min(len(lines), start+h)], "\n")
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Height(h).Render(body),
		kit.Hints(m.width, [2]string{"↑↓", "rolar"}, [2]string{"tab", "próxima aba"}, [2]string{"?", "ajuda"}))
}

func (m Agents) dashboard() string {
	if len(m.agents) == 0 {
		return kit.StHint.Render("detectando agentes…")
	}
	var installed, missing []agent.Agent
	for _, ag := range m.agents {
		if ag.Installed {
			installed = append(installed, ag)
		} else {
			missing = append(missing, ag)
		}
	}
	cardW, perRow := (m.width-2)/2, 2
	if m.width < 90 { // coluna única em telas estreitas
		cardW, perRow = m.width, 1
	}
	rows := []string{kit.StTitle.Render("Seus agentes") + kit.StHint.Render(fmt.Sprintf("  %d de %d instalados", len(installed), len(m.agents))), ""}
	for _, row := range m.cards(installed, cardW, perRow) {
		rows = append(rows, row, "")
	}
	if len(missing) > 0 {
		// Não instalados não têm o que mostrar: uma linha em vez de cards vazios.
		parts := make([]string, len(missing))
		for i, ag := range missing {
			parts[i] = kit.StOff.Render("○ ") + kit.CardLabel.Render(ag.Name)
			if ag.Detail != "" && ag.Detail != "não instalado" {
				parts[i] += kit.StHint.Render(" (" + ag.Detail + ")")
			}
		}
		line := kit.StHint.Render("não instalados   ") + strings.Join(parts, kit.StHint.Render("   "))
		rows = append(rows, lipgloss.NewStyle().Width(max(1, m.width)).Render(line))
	}
	return strings.TrimRight(lipgloss.JoinVertical(lipgloss.Left, rows...), "\n")
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
