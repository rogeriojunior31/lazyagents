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
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Tab é a aba de diagnóstico dos agentes detectados: uma tabela com estado,
// versão, contagens e o que o lazyagents gerencia em cada um (instalados
// primeiro) e, embaixo, os diretórios e avisos do agente selecionado.
type Tab struct {
	home     string
	adapters []agent.Adapter

	agents      []agent.Agent // ordenados: instalados primeiro
	counts      map[string]int
	skillCounts map[string]int

	cursor        int
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
		if kit.DetailScroll[msg.String()] && len(m.agents) > 0 {
			m.scrollDetail(kit.DetailScrollMsg(msg))
			return m
		}
		switch msg.String() {
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
		sp := m.split()
		if row := msg.Y - tableTop; msg.Button == tea.MouseLeft && msg.Y < sp.ListH && row >= 0 && row < len(m.agents) {
			start, _ := kit.Window(m.cursor, len(m.agents), sp.ListH-tableTop)
			m.cursor = min(start+row, len(m.agents)-1)
		}
	case tea.MouseWheelMsg:
		if msg.Y >= m.split().ListH && len(m.agents) > 0 {
			m.scrollDetail(msg) // roda sobre o detalhe rola ele
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

// tableTop é o título mais a linha de cabeçalho das colunas.
const tableTop = 2

// fullTable é a largura a partir da qual as capacidades têm colunas próprias;
// abaixo dela saem da tabela e aparecem no detalhe.
const fullTable = 75

func (m Tab) bodyHeight() int { return max(6, m.height-1) }

// split põe a tabela inteira no topo (são poucos agentes: é o conteúdo
// principal) e dá o resto da altura aos diretórios e avisos.
func (m Tab) split() kit.Split {
	h := m.bodyHeight()
	listH := min(tableTop+len(m.agents)+1, max(tableTop+1, h-3))
	return kit.Split{ListW: m.width, ListH: listH, DetailW: m.width, DetailH: h - listH}
}

// View limita tudo à largura da aba: rede de segurança para terminal estreito.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if len(m.agents) == 0 {
		return kit.StHint.Render("  detecting agents…")
	}
	sp := m.split()
	return lipgloss.JoinVertical(lipgloss.Left, m.tableView(sp.ListW, sp.ListH), m.detailView(sp),
		kit.Hints(m.width, [2]string{"↑↓", "agent"}, [2]string{"shift+↑↓", "detail"}, [2]string{"?", "help"}))
}

// Colunas da tabela; as de capacidade só existem a partir de fullTable.
const (
	colAgent = iota
	colVersion
	colSkills
	colSessions
	colHooks
	colProvider
	colUsage
)

func (m Tab) tableCols(width int) []kit.Column {
	nameW, verW := 8, 7
	for _, ag := range m.agents {
		nameW = max(nameW, 2+lipgloss.Width(ag.Name))
		verW = max(verW, lipgloss.Width(ag.Version))
	}
	cols := []kit.Column{
		{Title: "agent", Width: min(nameW, 18)},
		{Title: "version", Width: min(verW, 12)},
		{Title: "skills", Width: 6, Align: lipgloss.Right},
		{Title: "sessions", Width: 8, Align: lipgloss.Right},
		{Title: "hooks", Width: 5, Align: lipgloss.Center},
		{Title: "provider", Width: 8, Align: lipgloss.Center},
		{Title: "usage", Width: 5, Align: lipgloss.Center},
	}
	if width < fullTable {
		cols = cols[:colHooks]
	}
	if width < 50 {
		cols[colVersion] = kit.Column{}
	}
	return cols
}

// caps diz o que o lazyagents sabe fazer com o agente, pelas interfaces
// opcionais que o adapter implementa.
func (m Tab) caps(ag agent.Agent) (hooks, provider, usage bool) {
	var ad agent.Adapter
	for _, a := range m.adapters {
		if a.ID() == ag.ID {
			ad = a
		}
	}
	_, hooks = ad.(agent.HooksHost)
	_, provider = ad.(agent.ProviderHost)
	_, limits := ad.(agent.RateLimitReader)
	_, events := ad.(agent.UsageEventReader)
	return hooks, provider, limits || events
}

func check(ok bool) string {
	if ok {
		return kit.StOn.Render("✓")
	}
	return kit.StOff.Render("·")
}

// cells é a linha de um agente. Ausente sai esmaecido e sem números.
func (m Tab) cells(ag agent.Agent) []string {
	if !ag.Installed {
		none := kit.StOff.Render("—")
		return []string{kit.StOff.Render("○ " + ag.Name), kit.StOff.Render("missing"), none, none}
	}
	skills := kit.StOff.Render("—")
	if ag.SupportsSkills() {
		skills = fmt.Sprintf("%d", m.skillCounts[ag.ID])
	}
	hooks, provider, usage := m.caps(ag)
	return []string{
		lipgloss.NewStyle().Foreground(theme.AgentColor(ag.ID)).Render("● " + ag.Name),
		kit.StHint.Render(ag.Version), skills, fmt.Sprintf("%d", m.counts[ag.ID]),
		check(hooks), check(provider), check(usage),
	}
}

func (m Tab) tableView(w, h int) string {
	title := kit.StTitle.Render("AGENTS") + kit.StHint.Render(fmt.Sprintf("  %d of %d installed", m.InstalledCount(), len(m.agents)))
	cols := m.tableCols(w)
	lines := []string{"  " + title, kit.TableHeader(w, cols)}
	start, end := kit.Window(m.cursor, len(m.agents), max(1, h-tableTop))
	for i := start; i < end; i++ {
		lines = append(lines, kit.TableRow(w, i == m.cursor, cols, m.cells(m.agents[i])...))
	}
	return kit.Frame(strings.Join(lines, "\n"), "", h)
}

// detailViewport monta o viewport do detalhe na posição de leitura atual.
func (m Tab) detailViewport(sp kit.Split) viewport.Model {
	w, h := kit.DetailSize(sp)
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(ansi.Wrap(m.detailContent(m.agents[m.cursor], w), w, ""))
	vp.SetYOffset(m.detailOff)
	return vp
}

func (m Tab) detailView(sp kit.Split) string {
	return kit.DetailView(sp, strings.ToUpper(m.agents[m.cursor].Name), m.detailViewport(sp))
}

func (m *Tab) scrollDetail(msg tea.Msg) {
	vp, _ := m.detailViewport(m.split()).Update(msg)
	m.detailOff = vp.YOffset()
}

// detailContent traz o que não cabe na tabela, do mais curto ao mais longo:
// capacidades (em tela estreita), diretórios, detecção e avisos.
func (m Tab) detailContent(ag agent.Agent, inner int) string {
	var b strings.Builder
	if !ag.Installed {
		if ag.Detail != "" && ag.Detail != agent.DetailNotInstalled {
			b.WriteString(wrap(kit.StHint.Render(ag.Detail), inner) + "\n")
		}
		b.WriteString(wrap(kit.StHint.Render("Not installed. Install the CLI and reopen lazyagents to see it here."), inner))
		return b.String()
	}
	if m.width < fullTable {
		hooks, provider, usage := m.caps(ag)
		b.WriteString(field("manages", strings.Join([]string{
			check(hooks) + " hooks", check(provider) + " provider", check(usage) + " usage"}, "  "), inner))
	}
	if ag.SupportsSkills() {
		b.WriteString(field("skills in", core.Tilde(ag.ManagedDir, m.home), inner))
		label := "reads too"
		for _, d := range ag.ReadDirs {
			if d == ag.ManagedDir {
				continue
			}
			b.WriteString(field(label, core.Tilde(d, m.home), inner))
			label = "" // o rótulo só na primeira linha
		}
	} else {
		b.WriteString(field("skills", "no local skills dir", inner))
	}
	if ag.Detail != "" {
		b.WriteString(field("detection", ag.Detail, inner))
	}
	if ag.SharedNote != "" {
		b.WriteString(wrap(kit.StLocal.Render("⚠ "+ag.SharedNote), inner) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func field(label, value string, inner int) string {
	// Caminho comprido quebra alinhado à coluna do valor, sem perder o fim.
	wrapped := kit.Wrap(kit.CardValue.Render(value), inner, strings.Repeat(" ", 10))
	return kit.CardLabel.Render(fmt.Sprintf("%-10s", label)) + strings.TrimPrefix(wrapped, strings.Repeat(" ", 10)) + "\n"
}

func wrap(s string, width int) string { return lipgloss.NewStyle().Width(max(1, width)).Render(s) }

// --- module.Module ---

func (m *Tab) ID() string    { return "agents" }
func (m *Tab) Title() string { return "Agents" }

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
