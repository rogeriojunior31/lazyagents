package sessions

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type sessionItem struct {
	s      agent.Session
	live   bool // agent process still running
	marked bool // selected for a batch action (space)
}

// Title is the alias, if any, plus the prompt.
func (i sessionItem) Title() string {
	if i.s.Alias != "" {
		return i.s.Alias + "  " + i.s.Title
	}
	return i.s.Title
}

// FilterValue leaves out the full CWD on purpose: fuzzy matching letters
// scattered across paths made the filter useless. The basename is enough to
// filter by project.
func (i sessionItem) FilterValue() string {
	v := kit.AgentLabel(i.s.AgentID) + " " + i.s.Title
	if i.s.Alias != "" {
		v = kit.AgentLabel(i.s.AgentID) + " " + i.s.Alias + " " + i.s.Title
	}
	if base := filepath.Base(i.s.CWD); base != "" && base != "." {
		v += " " + base
	}
	return v
}

// sessionGroupHeader is a group row. It holds no session, so actions that
// assert sessionItem (enter, v, d…) ignore it; only space handles it.
type sessionGroupHeader struct {
	label string   // "▸ lazyagents · claude (12)"
	ids   []string // session IDs in the group
}

func (h sessionGroupHeader) Title() string { return h.label }

func (h sessionGroupHeader) FilterValue() string { return h.label }

// projectOf is the CWD basename, or "no project" for an empty or root CWD.
func projectOf(s agent.Session) string {
	base := filepath.Base(s.CWD)
	if s.CWD == "" || base == "" || base == "." || base == "/" || base == string(filepath.Separator) { // "\" is the Windows root
		return "no project"
	}
	return base
}

const (
	colMark = iota
	colAgent
	colTitle
	colProject
	colWhen
)

// narrowTable is the width below which the table shows only the agent color
// and hides the project (still in the detail and the filter).
const narrowTable = 64

// tableCols: mark (✓ selected, ● live), agent, session (flex), project, age.
func (m Tab) tableCols(width int) []kit.Column {
	agentW, projW, markW := 6, 7, 0
	for _, it := range m.list.Items() {
		if it, ok := it.(sessionItem); ok {
			agentW = max(agentW, 2+lipgloss.Width(kit.AgentLabel(it.s.AgentID)))
			projW = max(projW, lipgloss.Width(projectOf(it.s)))
			if it.marked || it.live {
				markW = 1 // the mark column only takes space when something is marked
			}
		}
	}
	cols := []kit.Column{
		{Width: markW},
		{Title: "agent", Width: agentW},
		{Title: "session", Flex: true},
		{Title: "project", Width: min(projW, 18)},
		{Title: "when", Width: 10, Align: lipgloss.Right},
	}
	if width < narrowTable {
		cols[colAgent] = kit.Column{Width: 1}
		cols[colProject] = kit.Column{}
	}
	return cols
}

func (m Tab) cells(it sessionItem, width int) []string {
	mark := ""
	switch {
	case it.marked:
		mark = kit.StShared.Render("✓")
	case it.live:
		mark = kit.StOn.Render("●")
	}
	tag := lipgloss.NewStyle().Foreground(theme.AgentColor(it.s.AgentID))
	agentCell := tag.Render("● " + kit.AgentLabel(it.s.AgentID))
	if width < narrowTable {
		agentCell = tag.Render("●")
	}
	title := it.s.Title
	if it.s.Alias != "" {
		title = kit.StTitle.Render(it.s.Alias) + "  " + kit.StHint.Render(it.s.Title)
	}
	return []string{mark, agentCell, title, kit.StHint.Render(projectOf(it.s)), kit.StHint.Render(relTime(it.s.MTime))}
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dmin ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// humanCount: 12345 → 12.3k.
func humanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatUsage(u agent.Usage) string {
	parts := []string{humanCount(u.Input) + " in", humanCount(u.Output) + " out"}
	if cache := u.CacheRead + u.CacheWrite; cache > 0 {
		parts = append(parts, "cache "+humanCount(cache))
	}
	return strings.Join(parts, " · ")
}

func newSessionItem(s agent.Session, live bool) sessionItem {
	return sessionItem{s: s, live: live}
}

func (m Tab) loadCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		sessions, err := svc.List()
		return events.SessionsLoaded{Sessions: sessions, Err: err}
	}
}

// applyItems rebuilds the list from the agent filter, the full-text search
// and m.grouped.
func (m *Tab) applyItems() tea.Cmd {
	var filtered []agent.Session
	for _, s := range m.sessions {
		if m.agentFilter != "" && s.AgentID != m.agentFilter {
			continue
		}
		if m.searchIDs != nil && !m.searchIDs[s.ID] {
			continue
		}
		filtered = append(filtered, s)
	}
	var items []list.Item
	if m.grouped {
		items = m.groupedItems(filtered)
	} else {
		items = m.flatItems(filtered)
	}
	cmd := m.list.SetItems(items)
	return tea.Batch(cmd, m.layout())
}

func (m *Tab) flatItems(sessions []agent.Session) []list.Item {
	items := make([]list.Item, 0, len(sessions))
	for _, s := range sessions {
		it := newSessionItem(s, m.svc.IsLive(s))
		it.marked = m.selected[s.ID]
		items = append(items, it)
	}
	return items
}

type sessionGroupKey struct{ agentID, project string }

// groupedItems groups by agent+project. sessions is already sorted by MTime,
// so groups follow their newest session and keep that order inside.
func (m *Tab) groupedItems(sessions []agent.Session) []list.Item {
	var order []sessionGroupKey
	byGroup := make(map[sessionGroupKey][]agent.Session)
	for _, s := range sessions {
		k := sessionGroupKey{s.AgentID, projectOf(s)}
		if _, ok := byGroup[k]; !ok {
			order = append(order, k)
		}
		byGroup[k] = append(byGroup[k], s)
	}
	items := make([]list.Item, 0, len(sessions)+len(order))
	for _, k := range order {
		group := byGroup[k]
		ids := make([]string, len(group))
		for i, s := range group {
			ids[i] = s.ID
		}
		label := fmt.Sprintf("▸ %s · %s (%d)", k.project, kit.AgentLabel(k.agentID), len(group))
		items = append(items, sessionGroupHeader{label: label, ids: ids})
		items = append(items, m.flatItems(group)...)
	}
	return items
}

// nextAgentFilter cycles all → each agent with sessions → all.
func (m Tab) nextAgentFilter() string {
	var cycle []string
	seen := map[string]bool{}
	for _, s := range m.sessions {
		if !seen[s.AgentID] {
			seen[s.AgentID] = true
			cycle = append(cycle, s.AgentID)
		}
	}
	if len(cycle) == 0 {
		return ""
	}
	for i, id := range cycle {
		if id == m.agentFilter {
			if i+1 < len(cycle) {
				return cycle[i+1]
			}
			return "" // back to all
		}
	}
	return cycle[0]
}
