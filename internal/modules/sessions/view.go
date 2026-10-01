package sessions

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// split divides the body; when stacked, height the table does not use goes to the detail.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight())
	if !sp.Side {
		need := len(m.tableHead(sp.ListW)) + max(1, len(m.list.VisibleItems())) + 1
		if spare := sp.ListH - need; spare > 0 {
			sp.ListH -= spare
			sp.DetailH += spare
		}
	}
	return sp
}

// refreshDetail keeps the scroll unless the selection changed, and returns the
// Cmd that loads the focused session's token usage, if not tried yet.
func (m *Tab) refreshDetail() tea.Cmd {
	w, h := kit.DetailSize(m.split())
	m.detailVP.SetWidth(w)
	m.detailVP.SetHeight(h)
	sel, ok := m.list.SelectedItem().(sessionItem)
	id := ""
	if ok {
		id = sel.s.ID
	}
	if id != m.detailID {
		m.detailVP.GotoTop()
		m.detailID = id
	}
	var cmd tea.Cmd
	if ok {
		cmd = m.maybeLoadUsageCmd(sel.s)
	}
	m.detailVP.SetContent(m.detailContent(w))
	return cmd
}

// maybeLoadUsageCmd loads usage lazily, on focus only, cached by ID: never at
// startup and never twice for the same session.
func (m *Tab) maybeLoadUsageCmd(s agent.Session) tea.Cmd {
	if _, tried := m.usageOK[s.ID]; tried {
		return nil
	}
	if m.usageBusy[s.ID] {
		return nil
	}
	if m.usageBusy == nil {
		m.usageBusy = make(map[string]bool)
	}
	m.usageBusy[s.ID] = true
	svc := m.svc
	return func() tea.Msg {
		u, ok := svc.SessionUsage(s)
		cost, hasCost := svc.SessionCost(s, svc.AuthMode(s.AgentID))
		return usageMsg{id: s.ID, usage: u, ok: ok, cost: cost, hasCost: ok && hasCost}
	}
}

// detailContent puts the resume command right under the title: it is what people come for.
func (m Tab) detailContent(inner int) string {
	if h, ok := m.list.SelectedItem().(sessionGroupHeader); ok {
		return kit.StHint.Render(fmt.Sprintf("%d session(s) in this group — ", len(h.ids))) +
			components.Keycap("space") + kit.StHint.Render(" selects all")
	}
	it, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return kit.StHint.Render("No sessions found.")
	}
	s := it.s
	label := func(l string) string { return kit.CardLabel.Render(fmt.Sprintf("%-8s", l)) }
	var b strings.Builder
	prose := lipgloss.NewStyle().Width(inner) // prose may wrap anywhere
	if s.Alias != "" {
		b.WriteString(prose.Render(kit.StTitle.Render(s.Alias)) + "\n" + prose.Render(kit.StHint.Render(kit.Truncate(s.Title, 200))) + "\n")
	} else {
		b.WriteString(prose.Render(kit.StTitle.Render(kit.Truncate(s.Title, 200))) + "\n")
	}
	if argv, dir, okCmd := m.svc.ResumeCmd(s); okCmd {
		// One line per command, wrapped only at spaces so the id and flags
		// stay whole for copying.
		b.WriteString(kit.CardLabel.Render("resume") + kit.StHint.Render("  enter · R in another folder") + "\n")
		for _, cmd := range []string{"cd " + core.Tilde(dir, m.home), strings.Join(argv, " ")} {
			b.WriteString(kit.MdCode.Render(wrapWords(cmd, max(8, inner))) + "\n")
		}
		if s.CWD != "" && dir != s.CWD {
			b.WriteString(prose.Render(kit.StWarn.Render("the session folder no longer exists; resumes in the folder above")) + "\n")
		}
	}
	st := lipgloss.NewStyle().Foreground(theme.AgentColor(s.AgentID))
	when := relTime(s.MTime) + kit.CardLabel.Render("  ("+s.MTime.Format("2006-01-02 15:04")+")")
	if m.svc.IsLive(s) {
		when += "  " + kit.StOn.Render("● open now")
	}
	b.WriteString("\n" + label("agent") + st.Render(s.AgentName) + "\n")
	b.WriteString(label("when") + kit.CardValue.Render(when) + "\n")
	if s.CWD != "" {
		b.WriteString(label("folder") + value(kit.CardValue.Render(core.Tilde(s.CWD, m.home)), inner) + "\n")
	}
	b.WriteString(label("id") + value(kit.CardLabel.Render(s.ID), inner) + "\n")
	if hasUsage, tried := m.usageOK[s.ID]; tried && hasUsage {
		u := m.usageCache[s.ID]
		if u.Input+u.Output+u.CacheRead+u.CacheWrite > 0 { // Crush records a cost only
			b.WriteString(label("tokens") + kit.CardValue.Render(formatUsage(u)) + "\n")
		}
		if cost, okCost := m.costCache[s.ID]; okCost {
			format := "~$%.2f" // an estimate from the price table
			if u.CostKnown {
				format = "$%.2f" // the agent's own figure
			}
			b.WriteString(label("cost") + kit.CardValue.Render(fmt.Sprintf(format, cost)) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// wrapWords wraps only at spaces so an argument (the session id) never
// splits; a word longer than the line keeps its own line.
func wrapWords(s string, width int) string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) <= width:
			line += " " + w
		default:
			lines, line = append(lines, line), w
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// value wraps at spaces only (paths and ids do not split at hyphens), indented to the value column.
func value(v string, inner int) string {
	pad := strings.Repeat(" ", 8)
	return strings.TrimPrefix(kit.Wrap(v, inner, pad), pad)
}

func (m Tab) readerView() string {
	s, st := m.docSession, m.docView.stats
	// Header in the same centered column as the chat, right of the rail.
	rail := m.railWidth()
	w, pad := chatColumn(m.width - 2 - rail)
	indent := strings.Repeat(" ", rail+pad)
	pos := fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)
	if m.vp.TotalLineCount() <= m.vp.VisibleLineCount() {
		pos = "all"
	}
	if m.docOpts.mode != modeLog {
		pos = modeNames[m.docOpts.mode] + " · " + pos
	}
	title := kit.StTitle.Render(ansi.Truncate(m.docTitle, max(10, w-lipgloss.Width(pos)-2), "…"))
	title = indent + title + strings.Repeat(" ", max(1, w-lipgloss.Width(title)-lipgloss.Width(pos))) + kit.StShared.Render(pos)

	meta := []string{lipgloss.NewStyle().Foreground(theme.AgentColor(s.AgentID)).Render(s.AgentName)}
	if s.CWD != "" {
		meta = append(meta, core.Tilde(s.CWD, m.home))
	}
	if !s.MTime.IsZero() {
		meta = append(meta, s.MTime.Format("2006-01-02 15:04"))
	}
	counts := fmt.Sprintf(plural(st.prompts, "%d prompt", "%d prompts"), st.prompts) + " · " +
		fmt.Sprintf(plural(st.tools, "%d command", "%d commands"), st.tools)
	if st.thoughts > 0 {
		counts += " · " + fmt.Sprintf(plural(st.thoughts, "%d thought", "%d thoughts"), st.thoughts)
	}
	meta = append(meta, counts)
	metaLine := indent + kit.StHint.Render(ansi.Truncate(strings.Join(meta, kit.StHint.Render(" · ")), w, "…"))

	keys := [][2]string{{"n/N", "prompt"}}
	if m.docOpts.mode == modeLog {
		steps, tools, thinking := "expand steps", "expand commands", "expand reasoning"
		if !m.docOpts.folded() {
			steps = "fold steps"
		}
		if m.docOpts.tools {
			tools = "collapse commands"
		}
		if m.docOpts.thinking {
			thinking = "collapse reasoning"
		}
		keys = append(keys, [2]string{"]/[ enter", "unfold · open"}, [2]string{"e", steps},
			[2]string{"t", tools}, [2]string{"r", thinking})
	}
	keys = append(keys, [2]string{"m", "next view"}, [2]string{"g/G", "top/end"}, [2]string{"x", "export"}, [2]string{"esc", "back"})
	body := m.vp.View()
	if rail > 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.railView(rail, m.vp.Height()), body)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, metaLine, body, kit.Hints(m.width, keys...), m.toastLine())
}

// railView lists the prompts beside the transcript, the one being read
// highlighted and kept in view; marks say the reply edited files (✎), had a
// failure (✗) or started subagents (⎇).
func (m Tab) railView(width, height int) string {
	items := m.docView.outline
	cur := m.currentPrompt()
	start := max(0, min(cur-height/2, len(items)-height))
	inner := width - 4 // a space before the rule
	var lines []string
	for i := start; i < len(items) && len(lines) < height; i++ {
		it := items[i]
		marks := ""
		if it.agents {
			marks += "⎇"
		}
		if it.edits {
			marks += kit.StAdded.Render("✎")
		}
		if it.failed {
			marks += kit.StErr.Render("✗")
		}
		head := fmt.Sprintf("%3d ", i+1)
		if !it.at.IsZero() {
			head += it.at.Local().Format("15:04") + " "
		}
		text := ansi.Truncate(it.text, max(1, inner-len(head)-lipgloss.Width(marks)-1), "…")
		gap := strings.Repeat(" ", max(1, inner-len(head)-lipgloss.Width(text)-lipgloss.Width(marks)))
		var line string
		if i == cur {
			line = kit.StShared.Bold(true).Render("▎"+head+text) + gap + marks
		} else {
			line = " " + kit.StHint.Render(head) + kit.StText.Render(text) + gap + marks
		}
		lines = append(lines, line)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, ln := range lines {
		lines[i] = ln + strings.Repeat(" ", max(0, width-2-lipgloss.Width(ln))) + kit.StHint.Render("│") + " "
	}
	return strings.Join(lines, "\n")
}

// toastLine renders the toast or the running spinner, in the list and the reader.
func (m Tab) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.inFlight {
		return m.spin.View() + " " + kit.StHint.Render(m.toast)
	}
	return components.Toast(m.toast, m.toastErr)
}

func (m Tab) View() string {
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.mode == sessModeDir {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(max(1, w-4)).Render("Working directory to resume in:"),
			"",
			components.InputView(m.dirInput, w-4),
			"",
			kit.Hints(w-4, [2]string{"enter", "confirm"}, [2]string{"esc", "back"}),
		)
		return components.Panel{Title: "Resume in folder", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeDoc {
		return m.readerView()
	}
	if m.mode == sessModeAlias {
		w := min(m.width, 72)
		content := lipgloss.JoinVertical(lipgloss.Left,
			kit.StHint.Render(kit.Truncate(m.aliasTarget.Title, w-6)),
			"",
			components.InputView(m.aliasInput, w-4),
			"",
			kit.StHint.Render("Leave empty to remove the alias.")+"\n"+kit.Hints(w-4, [2]string{"enter", "save"}, [2]string{"esc", "back"}),
		)
		return components.Panel{Title: "Session alias", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeSearch {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(max(1, w-4)).Render("Search the transcripts of every agent:"),
			"",
			components.InputView(m.searchInput, w-4),
			"",
			kit.Hints(w-4, [2]string{"enter", "search"}, [2]string{"esc", "back"}),
		)
		return components.Panel{Title: "Full-text search", Focused: true, Width: w}.Render(content)
	}
	sp := m.split()
	table := m.tableView(sp.ListW, sp.ListH)
	detail := kit.DetailView(sp, "CONTEXT", m.detailVP)
	body := lipgloss.JoinVertical(lipgloss.Left, table, detail)
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", detail)
	}
	hints := kit.Hints(m.width, [2]string{"enter", "resume"}, [2]string{"v", "transcript"},
		[2]string{"space", "select"}, [2]string{"f", "agent"}, [2]string{"g", "group"},
		[2]string{"F", "search"}, [2]string{"/", "filter"}, [2]string{"?", "help"})
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// tableHead: title with the active filters, the filter input and the column names.
func (m Tab) tableHead(w int) []string {
	title := kit.StTitle.Render("SESSIONS") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.list.VisibleItems())))
	if len(m.list.VisibleItems()) != len(m.sessions) {
		title += kit.StHint.Render(fmt.Sprintf(" of %d", len(m.sessions)))
	}
	if n := len(m.selectedSessions()); n > 0 {
		title += kit.StShared.Render(fmt.Sprintf(" · %d selected", n))
	}
	lines := []string{"  " + title}
	// Active filters stay visible: in the title if they fit, else on their own line.
	if f := m.filterSummary(); f != "" {
		if with := lines[0] + kit.StHint.Render(" · ") + kit.StLocal.Render(f); lipgloss.Width(with) <= w {
			lines[0] = with
		} else {
			lines = append(lines, "  "+kit.StLocal.Render(f))
		}
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(0, w), "…")
	}
	if m.list.FilterState() != list.Unfiltered {
		m.list.FilterInput.SetWidth(max(1, w-6))
		lines = append(lines, "  "+m.list.FilterInput.View())
	}
	return append(lines, kit.TableHeader(w, m.tableCols(w)))
}

// tableWindow is the visible row range and where it starts; rendering and clicks share it.
func (m Tab) tableWindow(w, h int) (start, end, top int) {
	top = len(m.tableHead(w))
	start, end = kit.Window(m.list.Index(), len(m.list.VisibleItems()), max(1, h-top))
	return start, end, top
}

func (m Tab) tableView(w, h int) string {
	lines := m.tableHead(w)
	items := m.list.VisibleItems()
	if len(items) == 0 {
		empty := "No sessions found."
		if m.list.FilterState() == list.FilterApplied {
			empty = fmt.Sprintf("Nothing found for “%s”.", m.list.FilterValue())
		}
		lines = append(lines, kit.StHint.Render("  "+empty))
	}
	cols := m.tableCols(w)
	group := []kit.Column{{Flex: true}}
	start, end, _ := m.tableWindow(w, h)
	for i := start; i < end; i++ {
		sel := i == m.list.Index()
		switch it := items[i].(type) {
		case sessionItem:
			lines = append(lines, kit.TableRow(w, sel, cols, m.cells(it, w)...))
		case sessionGroupHeader:
			lines = append(lines, kit.TableRow(w, sel, group, kit.StHint.Render(it.label)))
		}
	}
	return kit.Frame(strings.Join(lines, "\n"), "", h)
}

// --- module.Module ---

func (m Tab) filterSummary() string {
	var parts []string
	if m.agentFilter != "" {
		parts = append(parts, fmt.Sprintf("agent: %s", kit.AgentLabel(m.agentFilter)))
	}
	if m.searchIDs != nil {
		parts = append(parts, fmt.Sprintf("search: %q", m.searchQuery))
	}
	if m.grouped {
		parts = append(parts, "grouped")
	}
	return strings.Join(parts, " · ")
}
