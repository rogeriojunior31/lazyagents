package hooks

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// agentState is an entry's state in one agent.
type agentState int

const (
	stateOff         agentState = iota
	stateOn                     // every supported command installed
	statePartial                // some commands installed
	stateUnsupported            // the agent fires none of the entry's events
	stateMissing                // the CLI is not installed
)

func stateOf(st Status, h Hook) agentState {
	switch {
	case enabledIn(st, h.Name):
		return stateOn
	case partialIn(st, h.Name):
		return statePartial
	case !supportsAnyEvent(st, h):
		return stateUnsupported
	case !st.Installed:
		return stateMissing
	}
	return stateOff
}

// mark is the state marker and the style that colors it.
func (s agentState) mark() (string, lipgloss.Style) {
	switch s {
	case stateOn:
		return "●", kit.StOn
	case statePartial:
		return "◐", kit.StWarn
	case stateUnsupported:
		return "–", kit.StHint
	}
	return "○", kit.StOff
}

func (s agentState) label() string {
	switch s {
	case stateOn:
		return "installed"
	case statePartial:
		return "partial"
	case stateUnsupported:
		return "does not fire these events"
	case stateMissing:
		return "CLI not installed"
	}
	return "not installed"
}

// supportsAnyEvent reports whether the agent fires any of the entry's events;
// imported packages often mix events of different CLIs.
func supportsAnyEvent(st Status, h Hook) bool {
	for _, want := range h.Events() {
		for _, e := range st.Events {
			if strings.EqualFold(e, want) {
				return true
			}
		}
	}
	return false
}

// displayCommand shortens an imported command for the screen: drops the prefix
// that points the plugin root at the copy and shows the root as "./" (SOURCE
// already shows the folder).
func displayCommand(command string) string {
	cmd := stripRootExport(command)
	if cmd == command {
		return command
	}
	return strings.ReplaceAll(cmd, pluginRootSh+"/", "./")
}

// hooksTop is the title plus the table header row.
const hooksTop = 2

// split divides the body between table and detail; stacked, the table gets
// what it needs up to half the body, since a package's detail is long.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight())
	if !sp.Side {
		listH := min(hooksTop+max(1, len(m.lib))+1, max(hooksTop+1, m.bodyHeight()/2))
		sp.ListH, sp.DetailH = listH, m.bodyHeight()-listH
	}
	return sp
}

func (m Tab) listWidth() int { return m.split().ListW }

func (m Tab) listHeight() int { return m.split().ListH }

// bodyHeight is the body height minus hints and toast.
func (m Tab) bodyHeight() int { return max(6, m.height-2) }

// detailHeight is the detail panel height; when picking commands in the
// stacked layout it takes the whole body.
func (m Tab) detailHeight() int {
	sp := m.split()
	if m.cmdMode && !sp.Side {
		return m.bodyHeight()
	}
	return sp.DetailH
}

// View clamps everything to the tab width, a safety net for narrow terminals.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.reader != nil {
		return m.readerView()
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  reading the library and configs…")
	}

	bodyH := m.bodyHeight()
	var body string
	if len(m.lib) == 0 {
		p := components.Panel{Title: "HOOKS", Focused: true, Width: m.width, Height: bodyH}
		message := "No hooks installed.\n\nOpen Skills from the palette (: skills) and press i to import a repository with hooks."
		body = p.Render(lipgloss.NewStyle().Width(p.ContentWidth()).Render(message))
	} else if sp := m.split(); !sp.Side && m.cmdMode {
		body = m.detailPanel(m.width, bodyH)
	} else if !sp.Side {
		body = lipgloss.JoinVertical(lipgloss.Left,
			m.tableView(sp.ListW, sp.ListH),
			m.detailPanel(m.width, m.detailHeight()))
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.tableView(sp.ListW, sp.ListH), "  ",
			m.detailPanel(sp.DetailW, bodyH))
	}

	hints := kit.Hints(m.width,
		[2]string{"space", "install/uninstall"},
		[2]string{"←→", "agent"},
		[2]string{"enter", "commands"},
		[2]string{"v", "script"},
		[2]string{"a", "all"},
		[2]string{"x", "uninstall all"},
		[2]string{"?", "help"})
	if len(m.lib) == 0 {
		hints = kit.Hints(m.width, [2]string{":", "commands"}, [2]string{"?", "help"})
	} else if m.cmdMode {
		hints = kit.Hints(m.width,
			[2]string{"v", "script"},
			[2]string{"space", "toggle"},
			[2]string{"↑↓", "select"},
			[2]string{"pgup/pgdn", "read"},
			[2]string{"esc", "back"})
	}
	out := []string{body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// Hook table columns; agents start at colAgents.
const (
	colName = iota
	colCount
	colEvents
	colAgents
)

// statusAgents converts the tab's agents to the agent-column format.
func (m Tab) statusAgents() []agent.Agent {
	ags := make([]agent.Agent, len(m.statuses))
	for i, st := range m.statuses {
		ags[i] = agent.Agent{ID: st.AgentID, Name: st.AgentName, Short: st.Short}
	}
	return ags
}

// tableCols: hook, active commands, events (flex) and one column per agent,
// with the cursor column underlined.
func (m Tab) tableCols(width int) []kit.Column {
	agents := kit.AgentColumns(m.statusAgents(), width/3)
	for i, st := range m.statuses {
		if i == m.col {
			agents[i].Title = lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Underline(true).Bold(true).
				Render(ansi.Strip(agents[i].Title))
		}
	}
	nameW := 4
	for _, h := range m.lib {
		nameW = max(nameW, lipgloss.Width(h.Name))
	}
	cols := []kit.Column{
		{Title: "hook", Width: min(nameW, 24)},
		{Title: "cmds", Width: 5, Align: lipgloss.Right},
		{Title: "events", Flex: true},
	}
	return append(cols, agents...)
}

// cells is a hook's row: active/total commands, events and the state in each
// agent. On the selected row the cell under the cursor is inverted.
func (m Tab) cells(h Hook, selected bool) []string {
	count := fmt.Sprintf("%d", len(h.Hooks))
	if len(h.Off) > 0 {
		count = fmt.Sprintf("%d/%d", len(h.Active()), len(h.Hooks))
	}
	out := []string{h.Name, kit.StHint.Render(count), kit.StHint.Render(strings.Join(h.Events(), ", "))}
	for i, st := range m.statuses {
		mark, style := stateOf(st, h).mark()
		cell := style.Render(mark)
		if selected && i == m.col {
			cell = lipgloss.NewStyle().Reverse(true).Render(mark)
		}
		out = append(out, cell)
	}
	return out
}

// legend explains the matrix markers.
var legend = kit.StOn.Render("●") + kit.StHint.Render(" installed  ") +
	kit.StWarn.Render("◐") + kit.StHint.Render(" partial  ") +
	kit.StOff.Render("○") + kit.StHint.Render(" not installed  ") +
	kit.StHint.Render("– no such events")

// tableView is the library as a hook × agent matrix.
func (m Tab) tableView(w, h int) string {
	title := "  " + kit.StTitle.Render("LIBRARY") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.lib)))
	// The legend goes in the title if it fits, else at the table foot if a row is free.
	foot := ""
	if gap := w - lipgloss.Width(title) - lipgloss.Width(legend); gap >= 2 {
		title += strings.Repeat(" ", gap) + legend
	} else if h-hooksTop-len(m.lib) >= 2 {
		foot = "  " + legend
	}
	cols := m.tableCols(w)
	lines := []string{title, kit.TableHeader(w, cols)}
	start, end := kit.Window(m.cursor, len(m.lib), max(1, h-hooksTop))
	for i := start; i < end; i++ {
		lines = append(lines, kit.TableRow(w, i == m.cursor, cols, m.cells(m.lib[i], i == m.cursor)...))
	}
	return kit.Frame(strings.Join(lines, "\n"), foot, h)
}

func (m Tab) detailWidth() int { return m.split().DetailW }

// maxDetailOff is the detail's maximum scroll: the last full screen.
func (m Tab) maxDetailOff() int {
	p := components.Panel{Width: m.detailWidth()}
	return max(0, strings.Count(m.detailContent(p.ContentWidth()), "\n")+1-m.detailRows())
}

// detailPanel is the selected entry's card, scrollable with pgup/pgdn.
func (m Tab) detailPanel(w, h int) string {
	p := components.Panel{Title: "ABOUT THE HOOK", Focused: m.cmdMode, Width: w, Height: h}
	if m.cmdMode {
		if hook, ok := m.current(); ok {
			p.Title = fmt.Sprintf("COMMANDS · %d/%d", m.cmdCursor+1, len(hook.Hooks))
		}
	}
	lines := strings.Split(m.detailContent(p.ContentWidth()), "\n")
	visible := m.detailRows()
	off := min(m.detailOff, max(0, len(lines)-visible))
	lines = lines[off:]
	if len(lines) > visible && visible > 1 {
		lines = append(lines[:visible-1], kit.StHint.Render(fmt.Sprintf("↓ %d more lines · pgdn", len(lines)-visible+1)))
	}
	content := strings.Join(lines, "\n")
	if m.cmdMode {
		hook, _ := m.current()
		order := commandOrder(hook)
		start, end := kit.Window(m.cmdCursor, len(order), m.commandRows())
		var choices []string
		for i := start; i < end; i++ {
			idx := order[i]
			mark := "[x] "
			if hook.IsOff(idx) {
				mark = "[ ] "
			}
			prefix, style := "  ", kit.StText
			if i == m.cmdCursor {
				prefix = cmdCursorMark + " "
				style = style.Background(theme.Sel).Bold(true)
			}
			label := ansi.Truncate(prefix+mark+commandLabel(hook.Hooks[idx]), p.ContentWidth(), "…")
			choices = append(choices, style.Width(p.ContentWidth()).Render(label))
		}
		content = strings.Join(choices, "\n") + "\n\n" + content
	}
	return p.Render(content)
}

func (m Tab) commandRows() int {
	hook, _ := m.current()
	return min(len(hook.Hooks), 3, max(1, (m.detailHeight()-2)/3))
}

func (m Tab) detailRows() int {
	h := m.detailHeight() - 2
	if m.cmdMode {
		h -= m.commandRows() + 1
	}
	return max(1, h)
}

func (m Tab) detailContent(inner int) string {
	h, ok := m.current()
	if !ok {
		return strings.Join([]string{
			kit.StText.Render("No hooks in the library."),
			"",
			kit.StHint.Render("Create one from the CLI:"),
			kit.StText.Render(`  lazyagents hooks add doctor --event SessionStart --command "lazyagents doctor"`),
			"",
			kit.StHint.Render("Or install a plugin repository from the Skills tab (") + components.Keycap("i") +
				kit.StHint.Render("): its hooks/hooks.json comes along."),
		}, "\n")
	}
	if m.cmdMode {
		order := commandOrder(h)
		if m.cmdCursor >= len(order) {
			return ""
		}
		i := order[m.cmdCursor]
		c := h.Hooks[i]
		state := "enabled"
		if h.IsOff(i) {
			state = "disabled"
		}
		info := c.Event + " · " + state
		if c.Matcher != "" && c.Matcher != "*" {
			info += "\nMatcher: " + c.Matcher
		}
		if flags := commandFlags(c); flags != "" {
			info += " · " + flags
		}
		separator := "\n"
		if inner >= 40 {
			separator = "\n\n"
		}
		return ansi.Wrap(kit.StHint.Render(info)+separator+kit.StText.Render(c.Command), max(1, inner), "")
	}
	home := m.svc.home
	var b strings.Builder
	b.WriteString(kit.StTitle.Render(h.Name) + "\n")
	if h.Description != "" {
		b.WriteString(lipgloss.NewStyle().Width(inner).Render(kit.StText.Render(h.Description)) + "\n")
	}
	b.WriteString("\n" + kit.StHint.Render("IN AGENTS") + "\n")
	nameW := 0
	for _, st := range m.statuses {
		nameW = max(nameW, lipgloss.Width(st.AgentName))
	}
	for i, st := range m.statuses {
		s := stateOf(st, h)
		mark, style := s.mark()
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", nameW, st.AgentName))
		cursor := "  "
		if i == m.col {
			cursor = kit.StTitle.Render("▸ ") // the agent space toggles
		}
		b.WriteString(fmt.Sprintf("%s%s %s  %s\n", cursor, style.Render(mark), name, style.Render(s.label())))
		indent := strings.Repeat(" ", 6)
		var info []string
		if st.Err != "" {
			b.WriteString(kit.Wrap(kit.StErr.Render(st.Err), inner, indent) + "\n")
		}
		info = append(info, core.Tilde(st.File, home))
		if st.Foreign > 0 {
			info = append(info, fmt.Sprintf("%d foreign hook(s), left untouched", st.Foreign))
		}
		b.WriteString(kit.Wrap(kit.StHint.Render(strings.Join(info, " · ")), inner, indent) + "\n")
		if st.Note != "" {
			b.WriteString(kit.Wrap(kit.StWarn.Render(st.Note), inner, indent) + "\n")
		}
	}
	if h.Imported() || h.Files != "" {
		b.WriteString("\n" + kit.StHint.Render("SOURCE") + "\n")
		if h.Source != "" {
			b.WriteString(cardField("source", h.Source, inner))
		}
		if h.Files != "" {
			b.WriteString(cardField("root", core.Tilde(h.Files, home), inner))
		}
		if h.Imported() {
			b.WriteString(kit.Wrap(kit.StWarn.Render("! written for Claude Code; in another CLI the payload may differ"), inner, "") + "\n")
		}
	}
	if p := CommandProblem(h); p != "" {
		b.WriteString(kit.Wrap(kit.StErr.Render("⚠ "+p), inner, "") + "\n")
	}
	title := fmt.Sprintf("COMMANDS   %d", len(h.Hooks))
	if len(h.Off) > 0 {
		title = fmt.Sprintf("COMMANDS   %d of %d enabled", len(h.Active()), len(h.Hooks))
	}
	b.WriteString("\n" + kit.StHint.Render(title) + "\n")
	if !m.cmdMode {
		b.WriteString(kit.StHint.Render("enter picks which commands to install") + "\n")
	}
	b.WriteString(commandsBlock(h, inner))

	return strings.TrimRight(b.String(), "\n")
}

// cmdCursorMark marks the selected command.
const cmdCursorMark = "▸"

// commandOrder is the order of commands in the detail, grouped by event.
// Returns indexes into h.Hooks.
func commandOrder(h Hook) []int {
	var out []int
	for _, ev := range h.Events() {
		for i, c := range h.Hooks {
			if c.Event == ev {
				out = append(out, i)
			}
		}
	}
	return out
}

// commandLabel is a command's short name: the script it runs, or the shortened command.
func commandLabel(c agent.Hook) string {
	fields := strings.Fields(displayCommand(c.Command))
	for i := len(fields) - 1; i >= 0; i-- {
		f := strings.Trim(fields[i], `"'`)
		if strings.Contains(f, "/") {
			return filepath.Base(f)
		}
	}
	return kit.Truncate(displayCommand(c.Command), 40)
}

// commandsBlock keeps the summary compact; Enter opens the full reader.
func commandsBlock(h Hook, inner int) string {
	var lines []string
	for _, ev := range h.Events() {
		lines = append(lines, kit.StShared.Render(ev))
		for i, c := range h.Hooks {
			if c.Event != ev {
				continue
			}
			mark := "[x] "
			if h.IsOff(i) {
				mark = "[ ] "
			}
			lines = append(lines, kit.StText.Render(ansi.Truncate(mark+commandLabel(c), max(1, inner), "…")))
		}
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func commandFlags(h agent.Hook) string {
	var f []string
	if h.Async {
		f = append(f, "async")
	}
	if h.Timeout > 0 {
		f = append(f, fmt.Sprintf("%ds", h.Timeout))
	}
	return strings.Join(f, " · ")
}

// cardField is "label  value" with the value wrapped at its own column.
func cardField(label, value string, inner int) string {
	return kit.Field(label, kit.CardValue.Render(value), 9, inner) + "\n"
}
