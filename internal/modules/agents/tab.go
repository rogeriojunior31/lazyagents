package agents

import (
	"fmt"
	"path/filepath"
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

// Tab is the diagnostics tab: a table of the detected agents and, below it,
// the directories and warnings of the selected one.
type Tab struct {
	home     string
	adapters []agent.Adapter

	agents      []agent.Agent // installed first
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

// InstalledCount is the tab counter.
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
		if row := msg.Y - tableTop; msg.Button == tea.MouseLeft && msg.Y < sp.ListH && msg.X < sp.ListW && row >= 0 && row < len(m.agents) {
			start, _ := kit.Window(m.cursor, len(m.agents), sp.ListH-tableTop)
			m.cursor = min(start+row, len(m.agents)-1)
		}
	case tea.MouseWheelMsg:
		if sp := m.split(); (sp.Side && msg.X >= sp.ListW || !sp.Side && msg.Y >= sp.ListH) && len(m.agents) > 0 {
			m.scrollDetail(msg) // wheel over the detail scrolls it
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

// tableTop is the title plus the column header row.
const tableTop = 2

// fullTable is the width from which capabilities get their own columns;
// below it they move to the detail.
const fullTable = 75

func (m Tab) bodyHeight() int { return max(6, m.height-1) }

// split keeps the whole table on top (few agents, main content) and gives the
// rest of the height to the detail; on wide terminals the detail sits beside
// a table only as wide as its columns.
func (m Tab) split() kit.Split {
	h := m.bodyHeight()
	lw := 2 // row prefix
	for _, c := range m.tableCols(m.width) {
		lw += c.Width + 2
	}
	lw = max(lw, fullTable) // keeps the capability columns
	if m.width >= kit.SideDetailWidth && m.width-lw-2 >= 40 {
		return kit.Split{Side: true, ListW: lw, ListH: h, DetailW: m.width - lw - 2, DetailH: h}
	}
	listH := min(tableTop+len(m.agents)+1, max(tableTop+1, h-3))
	return kit.Split{ListW: m.width, ListH: listH, DetailW: m.width, DetailH: h - listH}
}

// View clamps to the tab width as a safety net for narrow terminals.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if len(m.agents) == 0 {
		return kit.StHint.Render("  detecting agents…")
	}
	sp := m.split()
	body := lipgloss.JoinVertical(lipgloss.Left, m.tableView(sp.ListW, sp.ListH), m.detailView(sp))
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.tableView(sp.ListW, sp.ListH), "  ", m.detailView(sp))
	}
	return lipgloss.JoinVertical(lipgloss.Left, body,
		kit.Hints(m.width, [2]string{"↑↓", "agent"}, [2]string{"shift+↑↓", "detail"}, [2]string{"?", "help"}))
}

// Capability columns only exist from fullTable up.
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

// caps reports what lazyagents manages, from the adapter's optional interfaces.
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

// cells renders an agent row; a missing agent is dimmed and has no counts.
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

// detailContent holds what does not fit the table: capabilities (narrow
// screens), directories, detection and warnings.
func (m Tab) detailContent(ag agent.Agent, inner int) string {
	var b strings.Builder
	// Detail is prose with paths inside (binary, config dir): Tilde only
	// shortens a whole path, so swap the home prefix wherever it appears
	detail := ag.Detail
	if m.home != "" {
		detail = strings.ReplaceAll(detail, m.home+string(filepath.Separator), "~"+string(filepath.Separator))
	}
	if !ag.Installed {
		if detail != "" && detail != agent.DetailNotInstalled {
			b.WriteString(wrap(kit.StHint.Render(detail), inner) + "\n")
		}
		b.WriteString(wrap(kit.StHint.Render("Not installed. Install the CLI and reopen lazyagents to see it here."), inner))
		return b.String()
	}
	if m.split().ListW < fullTable {
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
			value := core.Tilde(d, m.home)
			if d == ag.SharedDir {
				value += " (shared: used when every agent reading it has the skill)"
			}
			b.WriteString(field(label, value, inner))
			label = "" // label on the first line only
		}
	} else {
		b.WriteString(field("skills", "no local skills dir", inner))
	}
	if detail != "" {
		b.WriteString(field("detection", detail, inner))
	}
	return strings.TrimRight(b.String(), "\n")
}

func field(label, value string, inner int) string {
	return kit.Field(label, kit.CardValue.Render(value), 10, inner) + "\n"
}

func wrap(s string, width int) string { return lipgloss.NewStyle().Width(max(1, width)).Render(s) }

// --- module.Module ---

func (m *Tab) ID() string    { return "agents" }
func (m *Tab) Title() string { return "Agents" }

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	*m = m.step(msg)
	return nil
}

func (m *Tab) Count() int { return m.InstalledCount() }

// ClearToast is a no-op: this tab has no toast.
func (m *Tab) ClearToast() {}
