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

// transcriptView keeps the line of each user prompt so n/N can jump between them.
type transcriptView struct {
	content string
	prompts []int
	stats   transcriptStats
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
				line := "── " + e.Text + " ──"
				add(strings.Repeat(" ", max(0, (w-lipgloss.Width(line))/2)) + kit.StHint.Render(line))
			}
		case t.user:
			v.stats.prompts++
			v.prompts = append(v.prompts, len(lines))
			prompt = t.entries[len(t.entries)-1].Time
			label := kit.StHint.Render(fmt.Sprintf("#%d ", v.stats.prompts)) +
				lipgloss.NewStyle().Foreground(userColor).Bold(true).Render("You") + " "
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
			gutter(userColor, strings.Join(turnBlocks(t, w-2, o, &v.stats), "\n"))
		default:
			name := lipgloss.NewStyle().Foreground(agentColor).Bold(true).Render(agentName)
			if span := turnSpan(prompt, t); span > 0 {
				name += strings.Repeat(" ", max(1, w-lipgloss.Width(name)-len(span.String()))) + kit.StHint.Render(span.String())
			}
			prompt = time.Time{}
			add(name)
			gutter(agentColor, strings.Join(turnBlocks(t, w-2, o, &v.stats), "\n"))
		}
	}
	v.content = strings.Join(lines, "\n")
	return v
}

// transcriptOpts is what the reader expands.
type transcriptOpts struct {
	steps    bool   // e: every step of a turn instead of only its answer
	tools    bool   // t: one command per line instead of a summary
	thinking bool   // r: full reasoning instead of its first line
	home     string // shortens command paths to ~
}

// folded reports whether a turn shows only its answer: t and r would
// expand something hidden, so they unfold too.
func (o transcriptOpts) folded() bool { return !o.steps && !o.tools && !o.thinking }

var (
	thinkStyle = lipgloss.NewStyle().Foreground(theme.Subtle).Italic(true)
	cmdMark    = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
)

// turnBlocks wraps a turn's blocks to width in the agent's order, with a blank
// line whenever the kind changes so reasoning, text and commands stay apart.
// Folded, an agent turn is one steps line plus the messages written after its
// last command: narration before a command is progress, what follows the work
// is the answer (often several messages, as when a background task returns).
func turnBlocks(t turn, width int, o transcriptOpts, st *transcriptStats) []string {
	if !t.user && o.folded() {
		lastTool := -1
		for i, e := range t.entries {
			if e.Role == agent.RoleTool {
				lastTool = i
			}
		}
		var steps []agent.Entry
		var reply []string
		for i, e := range t.entries {
			if i > lastTool && e.Role == agent.RoleAssistant {
				reply = append(reply, "", kit.RenderChat(e.Text, width))
			} else {
				steps = append(steps, e)
			}
		}
		if len(steps) > 0 {
			return append([]string{stepsSummary(steps, width, st)}, reply...)
		}
	}
	var out []string
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
		st.tools += len(run)
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
			run = append(run, e)
			continue
		case agent.RoleThinking:
			flush()
			st.thoughts++
			emit(agent.RoleThinking, thinkingBlock(e.Text, width, o.thinking))
			continue
		}
		flush()
		emit(agent.RoleAssistant, kit.RenderChat(e.Text, width))
	}
	flush()
	return out
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

// toolLine: "❯ Bash  go test ./..."; a failed call ends in ✗.
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
	line := cmdMark.Render("❯ ") + kit.StShared.Bold(true).Render(name) + "  " + kit.MdCode.Render(arg)
	return ansi.Truncate(line, width-lipgloss.Width(mark), "…") + mark
}

// toolSummary: "❯ 4 commands · Bash ×3, Read · 1 failed"; a single command is shown whole.
func toolSummary(run []agent.Entry, home string, width int) string {
	if len(run) == 1 {
		return toolLine(run[0], home, width)
	}
	line := cmdMark.Render("❯ ") + kit.StShared.Render(fmt.Sprintf("%d commands", len(run))) +
		kit.StHint.Render(" · "+toolNames(run)) + failedNote(run)
	return ansi.Truncate(line, width, "…")
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
func stepsSummary(steps []agent.Entry, width int, st *transcriptStats) string {
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
	st.tools += len(calls)
	st.thoughts += thoughts
	var parts []string
	if msgs > 0 {
		parts = append(parts, fmt.Sprintf(plural(msgs, "%d message", "%d messages"), msgs))
	}
	if thoughts > 0 {
		parts = append(parts, fmt.Sprintf(plural(thoughts, "%d thought", "%d thoughts"), thoughts))
	}
	if len(calls) > 0 {
		parts = append(parts, fmt.Sprintf(plural(len(calls), "%d command: %s", "%d commands: %s"), len(calls), toolNames(calls)))
	}
	line := cmdMark.Render("⋯ ") + kit.StHint.Render(strings.Join(parts, " · ")) + failedNote(calls)
	return ansi.Truncate(line, width, "…")
}

// plural picks the format of a whole phrase, so a catalog can translate each form.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
