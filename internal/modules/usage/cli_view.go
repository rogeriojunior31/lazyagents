package usage

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Usage CLI text. Styles are the TUI's; lipgloss.Fprint downsamples colors
// to the terminal and strips them when output is not a terminal (pipe, file,
// NO_COLOR).

const cliBarW = 24 // limit and share bars

var viewTitles = map[string]string{
	"daily": "Tokens per day", "agents": "Tokens per agent",
	"projects": "Tokens per project", "models": "Tokens per model",
}

var viewHeads = map[string]string{
	"daily": "DAY", "agents": "AGENT", "projects": "PROJECT", "models": "MODEL",
}

// renderLimits lists each agent's limit windows.
func renderLimits(sts []Status) string {
	var b strings.Builder
	for i, st := range sts {
		if i > 0 {
			b.WriteString("\n")
		}
		meta := []string{st.AuthLabel}
		if st.Limits.Plan != "" {
			meta = append([]string{st.Limits.Plan}, meta...)
		}
		if st.Cached && !st.Limits.FetchedAt.IsZero() {
			meta = append(meta, "cached at "+st.Limits.FetchedAt.Local().Format("15:04"))
		}
		b.WriteString(agentName(st.AgentID) + "  " + kit.StHint.Render(strings.Join(meta, " · ")) + "\n")
		switch {
		case st.Err != "" && len(st.Limits.Windows) == 0:
			b.WriteString("  " + kit.StErr.Render("✗ "+st.Err) + "\n")
			continue
		case st.Err != "":
			b.WriteString("  " + kit.StWarn.Render(fmt.Sprintf("! %s — showing the cache", st.Err)) + "\n")
		case len(st.Limits.Windows) == 0:
			b.WriteString("  " + kit.StHint.Render("no subscription limits to show") + "\n")
		}
		for _, w := range st.Limits.Windows {
			fmt.Fprintf(&b, "  %-16s %s %5.1f%%  %s\n", kit.Truncate(w.Label, 16),
				bar(w.UsedPercent, cliBarW), w.UsedPercent, kit.StHint.Render(resetIn(w.ResetsAt)))
		}
	}
	return b.String()
}

// renderTotals is the table of an aggregated view (day, agent, project, model).
func renderTotals(o usageOpts, rows []Total, hidden int, total Total, showCost bool) string {
	var b strings.Builder
	b.WriteString(kit.StTitle.Render(viewTitles[o.view]) + kit.StHint.Render(" · "+o.period) + "\n\n")
	b.WriteString(totalsTable(o.view, rows, total, total.Tokens, showCost, colsFull, cliBarW, 28, agentName))
	if hidden > 0 {
		b.WriteString(kit.StHint.Render(fmt.Sprintf("  … %d more (--limit 0 shows all)", hidden)) + "\n")
	}
	if showCost && !total.Priced {
		b.WriteString(kit.StHint.Render("  — cost unavailable: subscription account or model without a price in the table") + "\n")
	}
	return b.String()
}

// How many columns fit: the TUI shrinks the table with the tab width.
const (
	colsFull    = iota // tokens, input, output, cache, responses, cost, share
	colsMedium         // tokens, responses, cost, share
	colsMinimal        // tokens, share
)

// totalsTable is a view's table, with foot as the total row and each row's
// share relative to whole tokens. Shared by the CLI and the tab (which filters
// rows: the footer sums the visible ones, the share stays over the whole period).
// name labels an agent row: the CLI shows the id (what --agent accepts), the
// tab the display name.
func totalsTable(view string, rows []Total, foot Total, whole int, showCost bool, cols, barW, labelW int, name func(id string) string) string {
	head := []string{viewHeads[view], "TOKENS"}
	if cols == colsFull {
		head = append(head, "INPUT", "OUTPUT", "CACHE")
	}
	if cols <= colsMedium {
		head = append(head, "RESPONSES")
		if showCost {
			head = append(head, "COST")
		}
	}
	head = append(head, "")
	cells := func(t Total, label string) []string {
		r := []string{label, compact(t.Tokens)}
		if cols == colsFull {
			r = append(r, compact(t.Usage.Input), compact(t.Usage.Output), compact(t.Usage.CacheRead+t.Usage.CacheWrite))
		}
		if cols <= colsMedium {
			r = append(r, fmt.Sprint(t.Events))
			if showCost {
				r = append(r, money(t))
			}
		}
		return r
	}
	var body [][]string
	for _, t := range rows {
		label := t.Label
		switch view {
		case "agents":
			label = name(t.Label)
		case "daily":
			label = dayLabel(t.Label)
		default:
			label = kit.Truncate(label, labelW)
		}
		body = append(body, append(cells(t, label), share(t.Tokens, whole, barW)))
	}
	return table(head, body, append(cells(foot, "total"), ""))
}

// renderPanel is `usage` without a view: limits, current block and the period summary.
func renderPanel(o usageOpts, sts []Status, events []agent.UsageEvent, price Pricer, showCost bool, now time.Time) string {
	var parts []string
	if len(sts) > 0 {
		parts = append(parts, strings.TrimRight(renderLimits(sts), "\n"))
	}
	if block, ok := Current(Blocks(events, now)); ok {
		left := block.End.Sub(now)
		parts = append(parts, kit.StTitle.Render("Current block")+"  "+kit.StHint.Render(fmt.Sprintf(
			"since %s · %dh%02dmin left · ", block.Start.Local().Format("15:04"),
			int(left.Hours()), int(left.Minutes())%60))+humanTokens(Tokens(block.Usage)))
	}

	total := Sum(events, price)
	var b strings.Builder
	days := fillDays(Daily(events, 0, price), o.from, now)
	summary := []string{humanTokens(total.Tokens), fmt.Sprintf("%d responses", total.Events)}
	if showCost {
		summary = append(summary, money(total))
	}
	b.WriteString(kit.StTitle.Render(upperFirst(o.period)) + "  ")
	if len(days) > 1 {
		b.WriteString(sparkline(days) + "  ")
	}
	b.WriteString(strings.Join(summary, kit.StHint.Render(" · ")))
	if len(events) == 0 {
		b.WriteString("\n  " + kit.StHint.Render("no usage recorded in transcripts"))
	}
	if agents := ByAgent(events, price); len(agents) > 0 {
		var rows [][]string
		for _, t := range agents {
			rows = append(rows, []string{agentName(t.Label), compact(t.Tokens), share(t.Tokens, total.Tokens, cliBarW)})
		}
		b.WriteString("\n" + strings.TrimRight(table(nil, rows, nil), "\n"))
	}
	if projs := ByProject(events, price); len(projs) > 0 {
		b.WriteString("\n\n" + kit.StTitle.Render("Top projects") + "\n")
		var rows [][]string
		for _, t := range projs[:min(3, len(projs))] {
			rows = append(rows, []string{kit.Truncate(t.Label, 28), compact(t.Tokens), share(t.Tokens, total.Tokens, cliBarW)})
		}
		b.WriteString(strings.TrimRight(table(nil, rows, nil), "\n"))
	}
	parts = append(parts, b.String())
	parts = append(parts, kit.StHint.Render("details: lazyagents usage limits|daily|agents|projects|models · help usage"))
	return strings.Join(parts, "\n\n") + "\n"
}

// table aligns columns by visible width (ANSI styles do not count): the first
// left, the rest right. head and foot are optional; foot comes after a rule.
func table(head []string, rows [][]string, foot []string) string {
	all := append([][]string{head}, rows...)
	all = append(all, foot)
	var w []int
	for _, r := range all {
		for i, cell := range r {
			if i == len(w) {
				w = append(w, 0)
			}
			w[i] = max(w[i], lipgloss.Width(cell))
		}
	}
	line := func(r []string) string {
		parts := make([]string, len(r))
		for i, cell := range r {
			pad := strings.Repeat(" ", w[i]-lipgloss.Width(cell))
			if i == 0 {
				parts[i] = cell + pad
			} else {
				parts[i] = pad + cell
			}
		}
		return "  " + strings.TrimRight(strings.Join(parts, "  "), " ")
	}
	var b strings.Builder
	if head != nil {
		b.WriteString(kit.StHint.Render(line(head)) + "\n")
	}
	for _, r := range rows {
		b.WriteString(line(r) + "\n")
	}
	if foot != nil {
		width := len(w) * 2
		for _, x := range w {
			width += x
		}
		b.WriteString(kit.StHint.Render("  "+strings.Repeat("─", width-2)) + "\n")
		b.WriteString(kit.StTitle.Render(line(foot)) + "\n")
	}
	return b.String()
}

// share is part's bar of the total, with the percentage.
func share(part, total, width int) string {
	frac := 0.0
	if total > 0 {
		frac = float64(part) / float64(total)
	}
	filled := int(frac*float64(width) + 0.5)
	if part > 0 && filled == 0 {
		filled = 1 // small usage still shows
	}
	return kit.StShared.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(theme.Border).Render(strings.Repeat("░", width-filled)) +
		fmt.Sprintf(" %3.0f%%", frac*100)
}

// sparkline draws a series of days relative to the largest.
func sparkline(days []Total) string {
	const ticks = "▁▂▃▄▅▆▇█"
	peak := 0
	for _, d := range days {
		peak = max(peak, d.Tokens)
	}
	var b strings.Builder
	for _, d := range days {
		if d.Tokens == 0 || peak == 0 {
			b.WriteString(kit.StOff.Render("·"))
			continue
		}
		i := min(7, d.Tokens*8/(peak+1))
		b.WriteString(kit.StShared.Render(string([]rune(ticks)[i])))
	}
	return b.String()
}

func agentName(id string) string {
	return lipgloss.NewStyle().Foreground(theme.AgentColor(id)).Bold(true).Render(id)
}

// dayLabel shows the day with the weekday: "Tue 09-23"; today is highlighted.
func dayLabel(day string) string {
	if day == time.Now().Format("2006-01-02") {
		return kit.StTitle.Render(dayText(day))
	}
	return dayText(day)
}

// dayText is the unstyled day label (also what the text filter matches).
func dayText(day string) string {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return day
	}
	return t.Format("Mon 01-02")
}

func money(t Total) string {
	if !t.Priced {
		return "—"
	}
	return fmt.Sprintf("$%.2f", t.Cost)
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
