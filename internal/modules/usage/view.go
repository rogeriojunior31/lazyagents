package usage

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// bar draws a limit window's percentage bar.
func bar(percent float64, width int) string {
	if width < 4 {
		width = 4
	}
	filled := int(percent / 100 * float64(width))
	filled = max(0, min(width, filled))
	style := kit.StOn
	switch {
	case percent >= 90:
		style = kit.StErr
	case percent >= 70:
		style = kit.StWarn
	}
	return style.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(theme.Border).Render(strings.Repeat("░", width-filled))
}

// resetIn formats the time left until the window resets.
func resetIn(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Until(t)
	if d <= 0 {
		return "resets now"
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf("resets in %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("resets in %dh%02dmin", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("resets %s", t.Local().Format("01-02 15:04"))
	}
}

// limitLines renders an agent's limits: a header with the name in the agent's
// color, auth and plan; one line per window (label, bar, percent, reset) and,
// if the query failed, the full error, wrapped.
func (m Tab) limitLines(st Status, w int, labelW int) string {
	badge := kit.StOn.Render(st.AuthLabel)
	if st.Auth == agent.AuthAPIKey {
		badge = kit.StWarn.Render(st.AuthLabel)
	} else if st.Auth == agent.AuthUnknown {
		badge = kit.StOff.Render(st.AuthLabel)
	}
	head := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render("● ") + m.agentLabel(st.AgentID) + "  " + badge
	if st.Limits.Plan != "" {
		head += kit.StHint.Render(" · ") + kit.StTitle.Render(st.Limits.Plan)
	}
	if st.Cached && !st.Limits.FetchedAt.IsZero() {
		head += kit.StHint.Render(fmt.Sprintf("  (cached at %s)", st.Limits.FetchedAt.Local().Format("15:04")))
	}
	lines := []string{head}
	const indent = "   "
	if st.Err != "" {
		lines = append(lines, indent+kit.StWarn.Render("! Refresh failed"), kit.Wrap(kit.StText.Render(st.Err), w, indent))
		if len(st.Limits.Windows) > 0 {
			lines = append(lines, indent+kit.StWarn.Render("previous limits kept"))
		}
		lines = append(lines, indent+kit.StHint.Render("r to retry"))
	} else if len(st.Limits.Windows) == 0 {
		lines = append(lines, indent+kit.StHint.Render("no subscription limits to show"))
	}
	for _, win := range st.Limits.Windows {
		pct := fmt.Sprintf("%5.1f%%", win.UsedPercent)
		reset := kit.StHint.Render(resetIn(win.ResetsAt))
		// One line: label · bar · % · reset; without room, the label goes above.
		barW := min(28, w-len(indent)-labelW-1-7-2-lipgloss.Width(reset))
		if barW >= 8 && lipgloss.Width(win.Label) <= labelW {
			lines = append(lines, indent+kit.CardLabel.Render(fmt.Sprintf("%-*s", labelW, win.Label))+" "+
				bar(win.UsedPercent, barW)+" "+pct+"  "+reset)
			continue
		}
		lines = append(lines, indent+kit.CardLabel.Render(win.Label),
			indent+bar(win.UsedPercent, max(4, min(28, w-len(indent)-8)))+" "+pct)
		if r := resetIn(win.ResetsAt); r != "" {
			lines = append(lines, indent+kit.StHint.Render(r))
		}
	}
	return strings.Join(lines, "\n")
}

func humanTokens(n int) string { return compact(n) + " tokens" }

// compact abbreviates a count: 1.2M, 15.3k, 999.
func compact(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// chip is a filter option; the active one is highlighted.
func chip(label string, on bool) string {
	if on {
		return lipgloss.NewStyle().Foreground(theme.OnAccent).Background(theme.Primary).Bold(true).Padding(0, 1).Render(label)
	}
	return kit.StHint.Padding(0, 1).Render(label)
}

// filterBar shows the filters and their keys: every option on one line when it
// fits, one line per filter when not, and only the active ones on narrow
// terminals.
func (m Tab) filterBar() string {
	var ps, vs []string
	for _, p := range periods {
		ps = append(ps, p.label)
	}
	for _, v := range tabViews {
		vs = append(vs, v.label)
	}
	ids := append([]string{""}, agentsIn(m.statuses, m.events)...)
	ags := make([]string, len(ids))
	cur := 0
	for i, id := range ids {
		ags[i] = m.name(id)
		if id == m.f.agent {
			cur = i
		}
	}
	text := ""
	if m.f.text != "" && !m.filtering { // while typing, the text is already in the footer input
		text = kit.StHint.Render("/ ") + kit.StTitle.Render(m.f.text) + kit.StHint.Render("  (esc clears)")
	}

	if m.width < 76 {
		agentName := ansi.Truncate(ags[cur], max(4, m.width-20), "…")
		lines := []string{kit.StHint.Render("p ") + chip(ps[m.f.period], true) + kit.StHint.Render(" a ") + chip(agentName, true), kit.StHint.Render("v ") + chip(vs[m.f.view], true)}
		if text != "" {
			lines = append(lines, text)
		}
		return strings.Join(lines, "\n")
	}

	group := func(key string, opts []string, active int) string {
		parts := []string{components.Keycap(key)}
		for i, o := range opts {
			parts = append(parts, chip(o, i == active))
		}
		return strings.Join(parts, "")
	}
	groups := []string{group("p", ps, m.f.period), group("a", ags, cur), group("v", vs, m.f.view)}
	// Pack the groups into lines: all three on one when they fit, otherwise as
	// many as fit per line.
	sep := kit.StHint.Render("  │  ")
	var lines []string
	line := ""
	for _, g := range groups {
		switch {
		case line == "":
			line = g
		case lipgloss.Width(line+sep+g) <= m.width:
			line += sep + g
		default:
			lines, line = append(lines, line), g
		}
	}
	lines = append(lines, line)
	if text != "" {
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

// name is the agent's display name ("" = all); the id without detection.
func (m Tab) name(id string) string {
	if id == "" {
		return "all"
	}
	if n := m.names[id]; n != "" {
		return n
	}
	return id
}

// agentLabel is the agent's name in its color (title and table).
func (m Tab) agentLabel(id string) string {
	return lipgloss.NewStyle().Foreground(theme.AgentColor(id)).Bold(true).Render(m.name(id))
}

// body builds the whole screen (before the scroll slice).
func (m Tab) body() string {
	if len(m.statuses) == 0 && len(m.events) == 0 {
		if m.loading {
			return kit.StHint.Render("loading usage…")
		}
		return lipgloss.NewStyle().Width(max(1, m.width)).Render(kit.StHint.Render("No usage data available.\nOpen a session in a supported agent and press r to refresh."))
	}
	now := time.Now()
	var parts []string

	var sts []Status
	api := map[string]bool{}
	for _, st := range m.statuses {
		if st.Auth == agent.AuthAPIKey {
			api[st.AgentID] = true
		}
		if m.f.agent == "" || st.AgentID == m.f.agent {
			sts = append(sts, st)
		}
	}
	if lim := m.limits(sts); lim != "" {
		parts = append(parts, lim)
	}

	events := m.f.apply(m.events, now)
	from := m.f.from(m.events, now)
	price := pricerFor(api)
	showCost := len(api) > 0 && (m.f.agent == "" || api[m.f.agent])
	whole := Sum(events, price)

	var sum strings.Builder
	sum.WriteString(kit.StTitle.Render(upperFirst(periods[m.f.period].label)) + "  ")
	if days := fillDays(Daily(events, 0, nil), from, now); len(days) > 1 {
		keep := max(7, min(len(days), m.width-50)) // the sparkline fits on the line
		sum.WriteString(sparkline(days[len(days)-min(keep, len(days)):]) + "  ")
	}
	facts := []string{humanTokens(whole.Tokens), fmt.Sprintf("%d responses", whole.Events)}
	if showCost {
		facts = append(facts, money(whole))
	}
	sum.WriteString(strings.Join(facts, kit.StHint.Render(" · ")))
	if block, ok := Current(Blocks(events, now)); ok {
		left := block.End.Sub(now)
		sum.WriteString("\n" + kit.StTitle.Render("Current block") + kit.StHint.Render(fmt.Sprintf(
			"  since %s · %dh%02dmin left · ", block.Start.Local().Format("15:04"),
			int(left.Hours()), int(left.Minutes())%60)) + humanTokens(Tokens(block.Usage)))
	}
	parts = append(parts, ansi.Wrap(sum.String(), max(1, m.width), ""))

	view := tabViews[m.f.view].id
	title := kit.StTitle.Render(viewTitles[view]) + kit.StHint.Render(" · "+periods[m.f.period].label)
	if m.f.agent != "" {
		title += kit.StHint.Render(" · ") + m.agentLabel(m.f.agent)
	}
	rows := m.f.rows(events, price, from, now)
	switch {
	case len(events) == 0:
		parts = append(parts, title+"\n\n"+kit.StHint.Render("  no usage recorded in this period"))
	case len(rows) == 0:
		parts = append(parts, title+"\n\n"+kit.StHint.Render(ansi.Wrap(fmt.Sprintf("No row contains %q.\nEsc clears the filter.", m.f.text), max(1, m.width), "")))
	default:
		foot := whole
		if m.f.text != "" {
			foot = sumTotals(rows, price != nil)
		}
		cols := colsFull
		switch {
		case m.width < 64:
			cols = colsMinimal
		case m.width < 100:
			cols = colsMedium
		}
		tbl := strings.TrimRight(totalsTable(view, rows, foot, whole.Tokens, showCost, cols, m.agentLabel), "\n")
		if showCost && !foot.Priced {
			tbl += "\n" + kit.StHint.Render("  — cost unavailable: subscription account or model without a price in the table")
		}
		parts = append(parts, title+"\n\n"+tbl)
	}
	return strings.Join(parts, "\n\n")
}

// limits is the subscription limits block: one agent after another, with bars
// aligned across all of them.
func (m Tab) limits(sts []Status) string {
	if len(sts) == 0 {
		return ""
	}
	labelW := 0
	for _, st := range sts {
		for _, win := range st.Limits.Windows {
			labelW = max(labelW, lipgloss.Width(win.Label))
		}
	}
	labelW = min(labelW, 20)
	blocks := []string{kit.StTitle.Render("Limits")}
	for _, st := range sts {
		blocks = append(blocks, m.limitLines(st, m.width, labelW))
	}
	return strings.Join(blocks, "\n")
}

// bodyLines is the body clipped to the usable width (cards and columns never
// overflow), as lines for scrolling.
func (m Tab) bodyLines() []string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	return strings.Split(clamp.Render(m.body()), "\n")
}

func (m Tab) View() string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	lines := m.lines
	if lines == nil { // View before any Update
		lines = m.bodyLines()
		m.bar = lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.filterBar())
	}
	h := m.contentHeight()
	start := min(m.scroll, max(0, len(lines)-h))
	visible := lines[start:min(len(lines), start+h)]
	if rest := len(lines) - start - h; rest > 0 && h > 1 {
		visible = append(visible[:h-1:h-1], kit.StHint.Render(fmt.Sprintf("  ↓ %d more lines · j/pgdn", rest+1)))
	}
	return kit.Frame(clamp.Render(strings.Join(visible, "\n")), m.bottom(), m.height)
}

// bottom is what stays pinned: filters, hints (or the text filter input) and
// the progress or notice line.
func (m Tab) bottom() string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	foot := kit.Hints(m.width, [2]string{"p", "period"}, [2]string{"a", "agent"}, [2]string{"←→", "view"},
		[2]string{"/", "filter"}, [2]string{"↑↓", "scroll"}, [2]string{"r", "refresh"}, [2]string{"?", "help"})
	if m.filtering {
		m.input.SetWidth(max(1, m.width-5))
		m.input.SetCursor(m.input.Position())
		foot = kit.StHint.Render("/ ") + m.input.View() + "\n" + kit.StHint.Render("enter keeps · esc clears")
	}
	bar := lipgloss.NewStyle().MaxHeight(max(1, m.height-4)).Render(m.bar)
	out := []string{bar, clamp.Render(foot)}
	if m.loading {
		out = append(out, kit.StHint.Render("… refreshing usage"))
	} else if m.toast != "" {
		out = append(out, ansi.Truncate(components.Toast(m.toast, m.toastErr), max(1, m.width), "…"))
	}
	return strings.Join(out, "\n")
}

// contentHeight is the scrollable height: what is left above the pinned bottom.
func (m Tab) contentHeight() int {
	return max(1, m.height-lipgloss.Height(m.bottom()))
}
