package agents

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Tab é a aba de visão geral dos agentes detectados: lista à esquerda
// (instalados primeiro) e, à direita, versão, contagens, diretórios de
// skills e o que o lazyagents sabe gerenciar em cada um.
type Tab struct {
	home     string
	adapters []agent.Adapter

	agents      []agent.Agent // ordenados: instalados primeiro
	counts      map[string]int
	skillCounts map[string]int

	cursor        int
	paneFocus     kit.PaneID
	detailOff     int
	width, height int
}

func newTab(home string, adapters []agent.Adapter) Tab {
	return Tab{home: home, adapters: adapters, counts: map[string]int{}, skillCounts: map[string]int{}}
}

func (m Tab) Init() tea.Cmd { return nil }

// InstalledCount conta os agentes detectados como instalados (contador da aba).
func (m Tab) InstalledCount() int {
	n := 0
	for _, ag := range m.agents {
		if ag.Installed {
			n++
		}
	}
	return n
}

func (m Tab) Capturing() bool { return false }

func (m Tab) step(msg tea.Msg) Tab {
	previous := m.cursor
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		if m.paneFocus == kit.PaneDetail && len(m.agents) > 0 && kit.DetailScrollKeys[msg.String()] {
			m.scrollDetail(msg)
			return m
		}
		switch msg.String() {
		case "left":
			m.paneFocus = kit.PaneList
		case "right":
			m.paneFocus = kit.PaneDetail
		case "down", "j":
			m.cursor++
		case "up", "k":
			m.cursor--
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.agents) - 1
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && m.width >= narrowWidth && msg.X >= m.listWidth()+2 && msg.Y >= 0 && msg.Y < m.bodyHeight() {
			m.paneFocus = kit.PaneDetail
			return m
		}
		if m.width < narrowWidth && m.paneFocus == kit.PaneDetail {
			return m
		}
		if msg.Button == tea.MouseLeft && msg.X < m.listWidth() && msg.Y < m.listHeight() {
			if i := kit.RowAt(msg.Y, m.cursor, len(m.agents), m.listHeight()); i >= 0 {
				m.cursor = i
				m.paneFocus = kit.PaneList
			}
		}
	case tea.MouseWheelMsg:
		if m.paneFocus == kit.PaneDetail && len(m.agents) > 0 {
			m.scrollDetail(msg)
			return m
		}
		switch msg.Button {
		case tea.MouseWheelDown:
			m.cursor++
		case tea.MouseWheelUp:
			m.cursor--
		}
	case events.AgentsDetected:
		var installed, missing []agent.Agent
		for _, ag := range msg.Agents {
			if ag.Installed {
				installed = append(installed, ag)
			} else {
				missing = append(missing, ag)
			}
		}
		m.agents = append(installed, missing...)
	case events.SessionsLoaded:
		counts := map[string]int{}
		for _, s := range msg.Sessions {
			counts[s.AgentID]++
		}
		m.counts = counts
	case events.SkillsScanned:
		m.skillCounts = msg.ActiveByAgent
	}
	m.cursor = max(0, min(m.cursor, len(m.agents)-1))
	if previous != m.cursor {
		m.detailOff = 0
	}
	return m
}

// narrowWidth é a largura abaixo da qual só o painel focado aparece.
const narrowWidth = 76

func (m Tab) listWidth() int {
	if m.width < narrowWidth {
		return m.width
	}
	return max(30, m.width*2/5)
}

func (m Tab) bodyHeight() int { return max(6, m.height-1) }

// listHeight ocupa toda a área; no modo estreito só o painel focado aparece.
func (m Tab) listHeight() int { return m.bodyHeight() }

// View limita tudo à largura da aba: rede de segurança para terminal estreito.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if len(m.agents) == 0 {
		return kit.StHint.Render("  detectando agentes…")
	}
	bodyH := m.bodyHeight()
	var body string
	if m.width < narrowWidth {
		body = m.listPanel(m.width, bodyH)
		if m.paneFocus == kit.PaneDetail {
			body = m.detailPanel(m.width, bodyH)
		}
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.listPanel(m.listWidth(), bodyH), "  ",
			m.detailPanel(max(24, m.width-m.listWidth()-2), bodyH))
	}
	return lipgloss.JoinVertical(lipgloss.Left, body,
		kit.Hints(m.width, [2]string{"←/→", "painel"}, [2]string{"?", "atalhos"}, [2]string{"↑↓", "navegar"}))
}

func (m Tab) listPanel(w, h int) string {
	p := components.Panel{
		Title:   fmt.Sprintf("AGENTES   %d de %d instalados", m.InstalledCount(), len(m.agents)),
		Focused: m.paneFocus == kit.PaneList, Width: w, Height: h,
	}
	per := max(1, (p.ContentHeight()-1)/3)
	start, end := kit.Window(m.cursor, len(m.agents), per)
	lines := []string{""}
	for i := start; i < end; i++ {
		ag := m.agents[i]
		mark := kit.StOff.Render("○")
		desc := "não instalado"
		if ag.Installed {
			mark = lipgloss.NewStyle().Foreground(theme.AgentColor(ag.ID)).Render("●")
			desc = fmt.Sprintf("%d skills ativas · %d sessões", m.skillCounts[ag.ID], m.counts[ag.ID])
		}
		lines = append(lines, strings.Split(kit.ListRow(p.ContentWidth(), i == m.cursor, ag.Name, mark, desc), "\n")...)
		lines = append(lines, "")
	}
	if end < len(m.agents) {
		lines[len(lines)-1] = kit.StHint.Render(fmt.Sprintf("  ↓ mais %d", len(m.agents)-end))
	}
	return p.Render(strings.Join(lines, "\n"))
}

func (m Tab) detailViewport(w, h int) (components.Panel, viewport.Model) {
	p := components.Panel{Title: "SOBRE O AGENTE", Focused: m.paneFocus == kit.PaneDetail, Width: w, Height: h}

	vp := viewport.New(viewport.WithWidth(p.ContentWidth()), viewport.WithHeight(max(1, p.ContentHeight())))
	vp.SetContent(ansi.Wrap(m.detailContent(m.agents[m.cursor], p.ContentWidth()), p.ContentWidth(), ""))
	vp.SetYOffset(m.detailOff)
	if vp.TotalLineCount() > vp.VisibleLineCount() {
		p.Title += fmt.Sprintf(" · %.0f%%", vp.ScrollPercent()*100)
	}
	return p, vp
}

func (m Tab) detailPanel(w, h int) string {
	p, vp := m.detailViewport(w, h)
	return p.Render(vp.View())
}

func (m *Tab) scrollDetail(msg tea.Msg) {
	w := m.width
	if w >= narrowWidth {
		w = max(24, m.width-m.listWidth()-2)
	}
	_, vp := m.detailViewport(w, m.bodyHeight())
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "home", "g":
			vp.GotoTop()
		case "end", "G":
			vp.GotoBottom()
		default:
			vp, _ = vp.Update(msg)
		}
	} else {
		vp, _ = vp.Update(msg)
	}
	m.detailOff = vp.YOffset()
}

func (m Tab) detailContent(ag agent.Agent, inner int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(theme.AgentColor(ag.ID)).Bold(true).Render(ag.Name) + "\n")
	if !ag.Installed {
		b.WriteString(kit.StOff.Render("○ não instalado") + "\n")
		if ag.Detail != "" && ag.Detail != "não instalado" {
			b.WriteString(wrap(kit.StHint.Render(ag.Detail), inner) + "\n")
		}
		b.WriteString("\n" + wrap(kit.StHint.Render("Instale o CLI e reabra o lazyagents para ele aparecer aqui."), inner))
		return b.String()
	}
	status := kit.StOn.Render("● instalado")
	if ag.Version != "" {
		status += kit.StHint.Render("  " + ag.Version)
	}
	b.WriteString(status + "\n\n")
	b.WriteString(kit.StTitle.Render(fmt.Sprintf("%d", m.skillCounts[ag.ID])) + kit.CardLabel.Render(" skills ativas    ") +
		kit.StTitle.Render(fmt.Sprintf("%d", m.counts[ag.ID])) + kit.CardLabel.Render(" sessões") + "\n")

	b.WriteString("\n" + kit.StHint.Render("SKILLS") + "\n")
	if ag.SupportsSkills() {
		b.WriteString(field("ativa em", core.Tilde(ag.ManagedDir, m.home), inner))
		label := "também lê"
		for _, d := range ag.ReadDirs {
			if d == ag.ManagedDir {
				continue
			}
			b.WriteString(field(label, core.Tilde(d, m.home), inner))
			label = "" // o rótulo só na primeira linha
		}
	} else {
		b.WriteString(kit.StOff.Render("sem diretório local de skills") + "\n")
	}
	if ag.SharedNote != "" {
		b.WriteString(wrap(kit.StLocal.Render("⚠ "+ag.SharedNote), inner) + "\n")
	}

	b.WriteString("\n" + kit.StHint.Render("O LAZYAGENTS GERENCIA") + "\n")
	b.WriteString(m.capabilities(ag) + "\n")
	return strings.TrimRight(b.String(), "\n")
}

// capabilities lista o que o lazyagents sabe fazer com o agente, pelas
// interfaces opcionais que o adapter implementa.
func (m Tab) capabilities(ag agent.Agent) string {
	var ad agent.Adapter
	for _, a := range m.adapters {
		if a.ID() == ag.ID {
			ad = a
		}
	}
	_, hooks := ad.(agent.HooksHost)
	_, provider := ad.(agent.ProviderHost)
	_, limits := ad.(agent.RateLimitReader)
	_, usage := ad.(agent.UsageEventReader)
	caps := []struct {
		name string
		ok   bool
	}{
		{"skills", ag.SupportsSkills()},
		{"hooks", hooks},
		{"provedor", provider},
		{"uso", usage || limits},
	}
	var out []string
	for _, c := range caps {
		if c.ok {
			out = append(out, kit.StOn.Render("✓ ")+kit.CardValue.Render(c.name))
		} else {
			out = append(out, kit.StOff.Render("· "+c.name))
		}
	}
	return strings.Join(out, "   ")
}

func field(label, value string, inner int) string {
	// Caminho comprido quebra alinhado à coluna do valor, sem perder o fim.
	wrapped := kit.Wrap(kit.CardValue.Render(value), inner, strings.Repeat(" ", 10))
	return kit.CardLabel.Render(fmt.Sprintf("%-10s", label)) + strings.TrimPrefix(wrapped, strings.Repeat(" ", 10)) + "\n"
}

func wrap(s string, width int) string { return lipgloss.NewStyle().Width(max(1, width)).Render(s) }

// --- module.Module ---

func (m *Tab) ID() string    { return "agents" }
func (m *Tab) Title() string { return "Agentes" }

// Update aplica a mensagem e guarda o novo estado (semântica de ponteiro do
// module.Module).
func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	*m = m.step(msg)
	return nil
}

// Count é o contador da aba: agentes instalados.
func (m *Tab) Count() int { return m.InstalledCount() }

// ClearToast: a aba Agentes não tem toast.
func (m *Tab) ClearToast() {}
