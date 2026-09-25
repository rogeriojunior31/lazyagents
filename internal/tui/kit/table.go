package kit

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Column is a table column. Width 0 hides it; the Flex column (first one only)
// takes the remaining width.
type Column struct {
	Title string
	Width int
	Flex  bool
	Align lipgloss.Position
}

const (
	cellGap   = "  "
	rowPrefix = 2 // "▎ " or two spaces
)

// selSGR opens the selected-row highlight. It is re-emitted after every inner
// reset so colored cells keep their color without punching through the
// background (a Render with a background stops at the first reset).
func selSGR() string {
	return ansi.Style{}.BackgroundColor(theme.Sel).ForegroundColor(theme.Primary).Bold().String()
}

// textSGR is the plain text color, also reopened after colored cells.
func textSGR() string { return ansi.Style{}.ForegroundColor(theme.Text).String() }

func widths(width int, cols []Column) []int {
	out := make([]int, len(cols))
	used, flex, shown := 0, -1, 0
	for i, c := range cols {
		if c.Flex && flex < 0 {
			flex = i
			shown++
			continue
		}
		if c.Width > 0 {
			out[i] = c.Width
			used += c.Width
			shown++
		}
	}
	if flex >= 0 {
		gaps := max(0, shown-1) * len(cellGap)
		out[flex] = max(0, width-rowPrefix-used-gaps)
	}
	return out
}

// fit cuts and pads a cell (may contain ANSI) to w columns.
func fit(cell string, w int, align lipgloss.Position) string {
	cell = ansi.Truncate(cell, w, "…")
	return lipgloss.PlaceHorizontal(w, align, cell)
}

func joinCells(width int, cols []Column, cells []string) string {
	ws := widths(width, cols)
	parts := make([]string, 0, len(cols))
	for i, c := range cols {
		// The Flex column shows even at 0 width so the header stays aligned.
		if ws[i] == 0 && !c.Flex {
			continue
		}
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts = append(parts, fit(cell, ws[i], c.Align))
	}
	return strings.Join(parts, cellGap)
}

// ColumnAt returns the column under x (relative to the row start) in a
// TableRow of that width, or -1 on the prefix, a gap or beyond.
func ColumnAt(width int, cols []Column, x int) int {
	ws := widths(width, cols)
	pos := rowPrefix
	for i, c := range cols {
		if ws[i] == 0 && !c.Flex {
			continue
		}
		if x >= pos && x < pos+ws[i] {
			return i
		}
		pos += ws[i] + len(cellGap)
	}
	return -1
}

// TableHeader draws column titles aligned with TableRow; pre-colored titles
// (agent names) keep their color.
func TableHeader(width int, cols []Column) string {
	titles := make([]string, len(cols))
	for i, c := range cols {
		titles[i] = c.Title
	}
	row := ansi.Truncate("  "+joinCells(width, cols, titles), max(0, width), "")
	return paint(row, ansi.Style{}.ForegroundColor(theme.Subtle).String())
}

// TableRow draws a row of exactly width columns. On the selected row the
// selection background covers the whole row without erasing cell colors.
func TableRow(width int, selected bool, cols []Column, cells ...string) string {
	width = max(0, width)
	prefix, on := "  ", textSGR()
	if selected {
		prefix, on = "▎ ", selSGR()
	}
	row := ansi.Truncate(prefix+joinCells(width, cols, cells), width, "")
	row += strings.Repeat(" ", max(0, width-lipgloss.Width(row)))
	return paint(row, on)
}

// paint applies sgr to the whole row, reopening it after every inner reset.
func paint(row, sgr string) string {
	row = strings.NewReplacer("\x1b[m", "\x1b[m"+sgr, "\x1b[0m", "\x1b[0m"+sgr).Replace(row)
	return sgr + row + "\x1b[m"
}

// AgentLabel is the short agent name used in tags and column headers.
func AgentLabel(id string) string {
	for _, suf := range []string{"-cli", "-code", "-agent"} {
		id = strings.TrimSuffix(id, suf)
	}
	return id
}

// AgentColumns makes one centered column per agent, titled with its short name
// in its color; if the names do not fit in room, the single letter
// (agent.Short) keeps the matrix readable on narrow terminals.
func AgentColumns(ags []agent.Agent, room int) []Column {
	cols := make([]Column, len(ags))
	total := 0
	for _, ag := range ags {
		total += lipgloss.Width(AgentLabel(ag.ID)) + len(cellGap)
	}
	short := total > room
	for i, ag := range ags {
		label := AgentLabel(ag.ID)
		if short && ag.Short != "" {
			label = ag.Short
		}
		cols[i] = Column{
			Title: lipgloss.NewStyle().Foreground(theme.AgentColor(ag.ID)).Render(label),
			Width: lipgloss.Width(label),
			Align: lipgloss.Center,
		}
	}
	return cols
}

// TableDelegate renders bubbles/list items as one-line TableRows; items without
// Cells() (group headers) show a dimmed Title(). It skips the default filter
// highlight, which slices by rune and would corrupt the cells' ANSI.
type TableDelegate struct {
	Cols func(width int) []Column
}

func (TableDelegate) Height() int                         { return 1 }
func (TableDelegate) Spacing() int                        { return 0 }
func (TableDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d TableDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if it, ok := item.(interface{ Cells() []string }); ok && d.Cols != nil {
		fmt.Fprint(w, TableRow(width, index == m.Index(), d.Cols(width), it.Cells()...))
		return
	}
	if it, ok := item.(interface{ Title() string }); ok {
		fmt.Fprint(w, StHint.Render(ansi.Truncate("  "+ansi.Strip(it.Title()), max(0, width), "…")))
	}
}

// DetailScroll are the keys that scroll a table tab's detail; PgUp/PgDn stay
// with the list, which may have hundreds of rows.
var DetailScroll = map[string]bool{"shift+up": true, "shift+down": true, "ctrl+u": true, "ctrl+d": true}

// DetailScrollMsg maps shift+↑/↓ to the viewport's line keys, which ignore the
// modifier.
func DetailScrollMsg(msg tea.KeyPressMsg) tea.KeyPressMsg {
	switch msg.String() {
	case "shift+up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "shift+down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	return msg
}

// DetailSize is the detail's text area: the side panel minus Panel padding, or
// the bottom strip minus the title line and indent.
func DetailSize(sp Split) (int, int) {
	if sp.Side {
		p := components.Panel{Width: sp.DetailW, Height: sp.DetailH}
		return p.ContentWidth(), p.ContentHeight()
	}
	return max(1, sp.DetailW-2), max(1, sp.DetailH-1)
}

// DetailView frames the detail (right panel or bottom strip), with the scroll
// position in the title when the text does not fit.
func DetailView(sp Split, title string, vp viewport.Model) string {
	if vp.TotalLineCount() > vp.Height() {
		title += fmt.Sprintf(" · %d%% · shift+↑↓", int(vp.ScrollPercent()*100))
	}
	if sp.Side {
		return components.Panel{Title: title, Width: sp.DetailW, Height: sp.DetailH}.Render(vp.View())
	}
	rule := StHint.Render("── " + title + " " + strings.Repeat("─", max(0, sp.DetailW-lipgloss.Width(title)-4)))
	body := lipgloss.NewStyle().PaddingLeft(2).Render(vp.View())
	return Frame(lipgloss.JoinVertical(lipgloss.Left, rule, body), "", sp.DetailH)
}
