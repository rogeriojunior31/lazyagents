package sessions

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// maxChatWidth caps the reading width on wide terminals.
const maxChatWidth = 100

// turn is one user message, a run of session events, or everything the agent
// said and called until the next prompt, in order.
type turn struct {
	user    bool
	event   bool
	entries []agent.Entry
}

func turns(entries []agent.Entry) []turn {
	var out []turn
	for _, e := range entries {
		user, event := e.Role == agent.RoleUser, e.Role == agent.RoleEvent
		if n := len(out); n > 0 && out[n-1].user == user && out[n-1].event == event {
			out[n-1].entries = append(out[n-1].entries, e)
			continue
		}
		out = append(out, turn{user: user, event: event, entries: []agent.Entry{e}})
	}
	return out
}

// turnSpan is how long the agent worked on a turn: from the prompt (or the
// turn's first entry) to its last entry. Zero when the agent records no time.
func turnSpan(prompt time.Time, t turn) time.Duration {
	start, end := prompt, t.entries[len(t.entries)-1].Time
	if start.IsZero() {
		start = t.entries[0].Time
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	return end.Sub(start).Round(time.Second)
}

// transcriptView keeps the line of each user prompt so n/N can jump between
// them, and of what ]/[ can pick: agent turns with steps to unfold and
// subagents to open.
type transcriptView struct {
	content string
	prompts []int
	outline []outlineItem
	folds   []fold
	stats   transcriptStats
}

// outlineItem is one prompt in the rail: its first line, its time, and marks
// for what the agent did in reply.
type outlineItem struct {
	text                  string
	at                    time.Time
	edits, failed, agents bool
}

// fold is a line enter acts on: an agent turn whose steps unfold on their own,
// or, when sub is set, a subagent call or another branch whose transcript opens.
type fold struct {
	line, turn int
	sub        agent.Entry
}

type transcriptStats struct{ prompts, tools, thoughts int }

// chatColumn returns the centered column width (up to maxChatWidth) and its left indent.
func chatColumn(width int) (w, pad int) {
	w = max(20, min(width, maxChatWidth))
	return w, max(0, (width-w)/2)
}

// renderTranscript renders a session as a log in a centered column: each
// prompt is a numbered header, each agent turn a gutter in the agent color
// whose steps fold behind its answer (see turnBlocks).
func renderTranscript(entries []agent.Entry, width int, s agent.Session, o transcriptOpts) transcriptView {
	var v transcriptView
	if len(entries) == 0 {
		v.content = kit.StHint.Render("(empty transcript or unknown format)")
		return v
	}
	w, pad := chatColumn(width)
	agentName := s.AgentName
	if agentName == "" {
		agentName = "agent"
	}
	userColor, agentColor := theme.Primary, theme.AgentColor(s.AgentID)

	var lines []string
	add := func(block string) {
		for _, ln := range strings.Split(block, "\n") {
			lines = append(lines, strings.Repeat(" ", pad)+ln)
		}
	}
	gutter := func(c color.Color, block string) {
		bar := lipgloss.NewStyle().Foreground(c).Render("│")
		for _, ln := range strings.Split(block, "\n") {
			if ln != "" {
				ln = bar + " " + ln
			} else {
				ln = bar
			}
			add(ln)
		}
	}
	var prompt time.Time // of the last prompt, for the turn's duration
	lastDay := ""        // the date shows only when the day changes
	for i, t := range turns(entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		switch {
		case t.event:
			for _, e := range t.entries {
				if e.Sub != "" { // another branch of the conversation, opened like a subagent
					v.folds = append(v.folds, fold{line: len(lines), turn: i, sub: e})
				}
				line := "── " + e.Text + " ──"
				add(strings.Repeat(" ", max(0, (w-lipgloss.Width(line))/2)) + kit.StHint.Render(line))
			}
		case t.user:
			v.stats.prompts++
			v.prompts = append(v.prompts, len(lines))
			first, _, _ := strings.Cut(strings.TrimSpace(t.entries[0].Text), "\n")
			v.outline = append(v.outline, outlineItem{text: first, at: t.entries[0].Time})
			prompt = t.entries[len(t.entries)-1].Time
			who := "You"
			if o.task {
				who = "Task"
			}
			label := kit.StHint.Render(fmt.Sprintf("#%d ", v.stats.prompts)) +
				lipgloss.NewStyle().Foreground(userColor).Bold(true).Render(who) + " "
			when := ""
			if at := t.entries[0].Time; !at.IsZero() {
				at = at.Local()
				when = " " + at.Format("15:04")
				if day := at.Format("2006-01-02"); day != lastDay {
					when, lastDay = " "+day+when, day
				}
			}
			rule := strings.Repeat("─", max(0, w-lipgloss.Width(label)-lipgloss.Width(when)))
			add(label + kit.StHint.Render(rule+when))
			blocks, _ := turnBlocks(t, w-2, o)
			gutter(userColor, strings.Join(blocks, "\n"))
		default:
			countTurn(t, &v)
			name := lipgloss.NewStyle().Foreground(agentColor).Bold(true).Render(agentName)
			if span := turnSpan(prompt, t); span > 0 {
				name += strings.Repeat(" ", max(1, w-lipgloss.Width(name)-len(span.String()))) + kit.StHint.Render(span.String())
			}
			prompt = time.Time{}
			add(name)
			if o.mode == modeLog && hasSteps(t) {
				v.folds = append(v.folds, fold{line: len(lines), turn: i})
			}
			to := o
			to.steps = o.steps || o.open[i]
			blocks, subs := turnBlocks(t, w-2, to)
			at := len(lines)
			for b, block := range blocks {
				if sub, ok := subs[b]; ok {
					v.folds = append(v.folds, fold{line: at, turn: i, sub: sub})
				}
				at += strings.Count(block, "\n") + 1
			}
			gutter(agentColor, strings.Join(blocks, "\n"))
		}
	}
	v.content = strings.Join(lines, "\n")
	return v
}

// countTurn adds an agent turn to the header counts and marks its prompt in the rail.
func countTurn(t turn, v *transcriptView) {
	var item *outlineItem
	if n := len(v.outline); n > 0 {
		item = &v.outline[n-1]
	}
	for _, e := range t.entries {
		switch e.Role {
		case agent.RoleThinking:
			v.stats.thoughts++
		case agent.RoleTool:
			v.stats.tools++
			if item != nil {
				item.edits = item.edits || e.Kind == agent.ToolEdit
				item.failed = item.failed || e.Failed
				item.agents = item.agents || e.Kind == agent.ToolAgent
			}
		}
	}
}

// Reading modes, cycled with m.
const (
	modeLog     = iota // answers, with the steps folded behind them
	modeChat           // prompts and answers only
	modeActions        // prompts and every call, no messages
)

var modeNames = []string{"log", "conversation", "actions"}

// transcriptOpts is what the reader expands.
type transcriptOpts struct {
	mode     int
	steps    bool         // e: every step of a turn instead of only its answer
	open     map[int]bool // enter: turns unfolded one by one
	tools    bool         // t: one command per line instead of a summary
	thinking bool         // r: full reasoning instead of its first line
	home     string       // shortens command paths to ~
	task     bool         // a subagent's transcript: its prompt is the parent agent's task
}

// folded reports whether a turn shows only its answer: t and r would
// expand something hidden, so they unfold too.
func (o transcriptOpts) folded() bool { return !o.steps && !o.tools && !o.thinking }

var (
	thinkStyle = lipgloss.NewStyle().Foreground(theme.Subtle).Italic(true)
	cmdMark    = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
)

// splitTurn separates an agent turn's answer from the steps towards it. The
// answer is the messages written after its last command (often several, as
// when a background task returns) plus any plan it proposed; narration before
// a command is progress.
func splitTurn(t turn) (steps, answer []agent.Entry) {
	lastTool := -1
	for i, e := range t.entries {
		if e.Role == agent.RoleTool {
			lastTool = i
		}
	}
	for i, e := range t.entries {
		if (i > lastTool && e.Role == agent.RoleAssistant) || (isDocument(e) && e.Kind == agent.ToolPlan) {
			answer = append(answer, e)
		} else {
			steps = append(steps, e)
		}
	}
	return steps, answer
}

func hasSteps(t turn) bool {
	steps, _ := splitTurn(t)
	return len(steps) > 0
}

// turnBlocks wraps a turn's blocks to width in the agent's order, with a blank
// line whenever the kind changes so reasoning, text and commands stay apart.
// Folded, an agent turn is its answer under one line summing up its steps; in
// conversation mode that line shows only when there is no answer, in actions
// mode the turn is its calls alone. subs maps a block to the subagent call it
// shows, for calls whose transcript can be opened.
func turnBlocks(t turn, width int, o transcriptOpts) (blocks []string, subs map[int]agent.Entry) {
	if !t.user && o.mode == modeActions {
		return actionBlocks(t, width, o.home)
	}
	if !t.user && (o.folded() || o.mode == modeChat) {
		steps, answer := splitTurn(t)
		if len(steps) > 0 || o.mode == modeChat {
			var out []string
			if len(steps) > 0 && (o.mode == modeLog || len(answer) == 0) {
				out = append(out, stepsSummary(steps, width))
			}
			for _, e := range answer {
				if len(out) > 0 {
					out = append(out, "")
				}
				if e.Role == agent.RoleTool {
					out = append(out, toolBlock(e, width))
				} else {
					out = append(out, kit.RenderChat(e.Text, width))
				}
			}
			return out, nil
		}
	}
	var out []string
	subs = map[int]agent.Entry{}
	var run []agent.Entry // pending consecutive commands
	last := ""            // role of the last emitted block
	emit := func(role, block string) {
		if last != "" && (role != last || role == agent.RoleAssistant) {
			out = append(out, "")
		}
		out, last = append(out, block), role
	}
	flush := func() {
		if len(run) == 0 {
			return
		}
		if o.tools {
			lines := make([]string, len(run))
			for i, call := range run {
				lines[i] = toolLine(call, o.home, width)
			}
			emit(agent.RoleTool, strings.Join(lines, "\n"))
		} else {
			emit(agent.RoleTool, toolSummary(run, o.home, width))
		}
		run = nil
	}
	for _, e := range t.entries {
		switch e.Role {
		case agent.RoleTool:
			if isDocument(e) {
				flush()
				emit(agent.RoleTool, toolBlock(e, width))
				continue
			}
			if e.Sub != "" { // its own line, so it can be picked and opened
				flush()
				emit(agent.RoleTool, toolLine(e, o.home, width))
				subs[len(out)-1] = e
				continue
			}
			run = append(run, e)
			continue
		case agent.RoleThinking:
			flush()
			emit(agent.RoleThinking, thinkingBlock(e.Text, width, o.thinking))
			continue
		}
		flush()
		emit(agent.RoleAssistant, kit.RenderChat(e.Text, width))
	}
	flush()
	return out, subs
}

// actionBlocks is a turn as its calls only, one per line, plans and task
// lists whole.
func actionBlocks(t turn, width int, home string) ([]string, map[int]agent.Entry) {
	var out []string
	subs := map[int]agent.Entry{}
	for _, e := range t.entries {
		switch {
		case isDocument(e):
			out = append(out, toolBlock(e, width))
		case e.Role == agent.RoleTool:
			if e.Sub != "" {
				subs[len(out)] = e
			}
			out = append(out, toolLine(e, home, width))
		}
	}
	if len(out) == 0 {
		return []string{kit.StHint.Render("no commands")}, nil
	}
	return out, subs
}

// thinkingBlock is dimmed: the first line plus "…" when collapsed, the full
// text behind a dotted rule when expanded.
func thinkingBlock(text string, width int, full bool) string {
	if !full {
		first, _, more := strings.Cut(strings.TrimSpace(text), "\n")
		line := "💭 " + first
		if more || lipgloss.Width(line) > width {
			line = ansi.Truncate(line, width-1, "") + "…"
		}
		return thinkStyle.Render(line)
	}
	body := lipgloss.NewStyle().Width(width - 2).Render(text)
	lines := []string{thinkStyle.Render("💭 reasoning")}
	for _, ln := range strings.Split(body, "\n") {
		lines = append(lines, thinkStyle.Render("┆ "+strings.TrimRight(ln, " ")))
	}
	return strings.Join(lines, "\n")
}

// isDocument: a plan or a task list, read as content rather than counted.
func isDocument(e agent.Entry) bool {
	return e.Role == agent.RoleTool && e.Body != "" && (e.Kind == agent.ToolPlan || e.Kind == agent.ToolTodo)
}

// toolLine shows a call by what it did: "$ go test", "✎ a.go +3 −1",
// a dimmed lookup, "⎇ Agent  task"; anything else "❯ Name  arg". A failed
// call ends in ✗.
func toolLine(call agent.Entry, home string, width int) string {
	text := call.Text
	if home != "" {
		text = strings.ReplaceAll(text, home+"/", "~/")
	}
	name, arg, _ := strings.Cut(text, " · ")
	mark := ""
	if call.Failed {
		mark = " " + kit.StErr.Render("✗")
	}
	var line string
	switch call.Kind {
	case agent.ToolShell:
		line = cmdMark.Render("$ ") + kit.MdCode.Render(arg)
	case agent.ToolEdit:
		mark = " " + lineCounts(call.Added, call.Removed) + mark
		line = kit.StAdded.Render("✎ ") + kit.StText.Render(arg)
	case agent.ToolRead:
		line = kit.StHint.Render("· " + name + "  " + arg)
	case agent.ToolAgent:
		line = cmdMark.Render("⎇ ") + kit.StShared.Bold(true).Render(name) + "  " + arg
		if call.Calls > 0 {
			mark = kit.StHint.Render(fmt.Sprintf(" · "+plural(call.Calls, "%d call", "%d calls"), call.Calls)) + mark
		}
	default:
		line = cmdMark.Render("❯ ") + kit.StShared.Bold(true).Render(name) + "  " + kit.MdCode.Render(arg)
	}
	return ansi.Truncate(line, max(1, width-lipgloss.Width(mark)), "…") + mark
}

// lineCounts: "+3 −1", leaving out a zero side.
func lineCounts(added, removed int) string {
	var parts []string
	if added > 0 {
		parts = append(parts, kit.StAdded.Render(fmt.Sprintf("+%d", added)))
	}
	if removed > 0 {
		parts = append(parts, kit.StRemoved.Render(fmt.Sprintf("−%d", removed)))
	}
	return strings.Join(parts, " ")
}

// toolBlock renders a plan as a document and a task list as its checklist,
// under a heading line.
func toolBlock(call agent.Entry, width int) string {
	head := cmdMark.Render("☰ ") + kit.StShared.Bold(true).Render("plan")
	body := kit.RenderChat(call.Body, width)
	if call.Kind == agent.ToolTodo {
		lines := strings.Split(call.Body, "\n")
		done := 0
		for _, ln := range lines {
			if strings.HasPrefix(ln, "☑") {
				done++
			}
		}
		head = cmdMark.Render("☑ ") + kit.StShared.Bold(true).Render("tasks") +
			kit.StHint.Render(fmt.Sprintf("  %d/%d done", done, len(lines)))
		for i, ln := range lines {
			lines[i] = ansi.Truncate(ln, width, "…")
		}
		body = strings.Join(lines, "\n")
	}
	if call.Failed {
		head += " " + kit.StErr.Render("✗")
	}
	return head + "\n" + body
}

// toolSummary: "❯ 4 commands · ✎ 2 files +9 −1 · Bash ×3, Edit"; a single
// command is shown whole.
func toolSummary(run []agent.Entry, home string, width int) string {
	if len(run) == 1 {
		return toolLine(run[0], home, width)
	}
	line := cmdMark.Render("❯ ") + kit.StShared.Render(fmt.Sprintf("%d commands", len(run))) +
		editsNote(run) + failedNote(run) + kit.StHint.Render(" · "+toolNames(run))
	return ansi.Truncate(line, width, "…")
}

// editsNote: " · ✎ 2 files +9 −1", or nothing when no call edited a file.
func editsNote(calls []agent.Entry) string {
	files := map[string]bool{}
	added, removed := 0, 0
	for _, c := range calls {
		if c.Kind != agent.ToolEdit {
			continue
		}
		_, arg, _ := strings.Cut(c.Text, " · ")
		for _, f := range strings.Split(arg, ", ") {
			files[f] = true
		}
		added += c.Added
		removed += c.Removed
	}
	if len(files) == 0 {
		return ""
	}
	note := fmt.Sprintf(plural(len(files), "%d file", "%d files"), len(files))
	if counts := lineCounts(added, removed); counts != "" {
		note += " " + counts
	}
	return kit.StHint.Render(" · ") + kit.StAdded.Render("✎ ") + kit.StHint.Render(note)
}

// toolNames: "Bash ×3, Read", in order of first use.
func toolNames(calls []agent.Entry) string {
	var order []string
	count := map[string]int{}
	for _, call := range calls {
		name, _, _ := strings.Cut(call.Text, " · ")
		if count[name] == 0 {
			order = append(order, name)
		}
		count[name]++
	}
	parts := make([]string, len(order))
	for i, n := range order {
		parts[i] = n
		if count[n] > 1 {
			parts[i] += fmt.Sprintf(" ×%d", count[n])
		}
	}
	return strings.Join(parts, ", ")
}

// failedNote: " · ✗ 2 failed", or nothing when every call succeeded.
func failedNote(calls []agent.Entry) string {
	n := 0
	for _, c := range calls {
		if c.Failed {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return kit.StHint.Render(" · ") + kit.StErr.Render(fmt.Sprintf("✗ %d failed", n))
}

// stepsSummary: "⋯ 2 messages · 1 thought · 3 commands: Bash ×2, Read".
func stepsSummary(steps []agent.Entry, width int) string {
	var msgs, thoughts int
	var calls []agent.Entry
	for _, e := range steps {
		switch e.Role {
		case agent.RoleTool:
			calls = append(calls, e)
		case agent.RoleThinking:
			thoughts++
		default:
			msgs++
		}
	}
	var parts []string
	if msgs > 0 {
		parts = append(parts, fmt.Sprintf(plural(msgs, "%d message", "%d messages"), msgs))
	}
	if thoughts > 0 {
		parts = append(parts, fmt.Sprintf(plural(thoughts, "%d thought", "%d thoughts"), thoughts))
	}
	if len(calls) > 0 {
		parts = append(parts, fmt.Sprintf(plural(len(calls), "%d command", "%d commands"), len(calls)))
	}
	line := cmdMark.Render("⋯ ") + kit.StHint.Render(strings.Join(parts, " · ")) + editsNote(calls) + failedNote(calls)
	if len(calls) > 0 {
		// names last: on a narrow screen they are what the truncation cuts
		line += kit.StHint.Render(" · " + toolNames(calls))
	}
	return ansi.Truncate(line, width, "…")
}

// plural picks the format of a whole phrase, so a catalog can translate each form.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
