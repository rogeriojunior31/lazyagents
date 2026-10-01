package sessions

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// maxChatWidth caps the reading width on wide terminals.
const maxChatWidth = 100

// turn is one user message, or everything the agent said and called until the
// next prompt, in order.
type turn struct {
	user    bool
	entries []agent.Entry
}

func turns(entries []agent.Entry) []turn {
	var out []turn
	for _, e := range entries {
		user := e.Role == agent.RoleUser
		if len(out) > 0 && out[len(out)-1].user == user {
			out[len(out)-1].entries = append(out[len(out)-1].entries, e)
			continue
		}
		out = append(out, turn{user: user, entries: []agent.Entry{e}})
	}
	return out
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
	for i, t := range turns(entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		if t.user {
			v.stats.prompts++
			v.prompts = append(v.prompts, len(lines))
			label := kit.StHint.Render(fmt.Sprintf("#%d ", v.stats.prompts)) +
				lipgloss.NewStyle().Foreground(userColor).Bold(true).Render("You") + " "
			rule := kit.StHint.Render(strings.Repeat("─", max(0, w-lipgloss.Width(label))))
			add(label + rule)
			gutter(userColor, strings.Join(turnBlocks(t, w-2, o, &v.stats), "\n"))
			continue
		}
		add(lipgloss.NewStyle().Foreground(agentColor).Bold(true).Render(agentName))
		gutter(agentColor, strings.Join(turnBlocks(t, w-2, o, &v.stats), "\n"))
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

// toolLine: "❯ Bash  go test ./...".
func toolLine(call agent.Entry, home string, width int) string {
	text := call.Text
	if home != "" {
		text = strings.ReplaceAll(text, home+"/", "~/")
	}
	name, arg, _ := strings.Cut(text, " · ")
	line := cmdMark.Render("❯ ") + kit.StShared.Bold(true).Render(name) + "  " + kit.MdCode.Render(arg)
	return ansi.Truncate(line, width, "…")
}

// toolSummary: "❯ 4 commands · Bash ×3, Read"; a single command is shown whole.
func toolSummary(run []agent.Entry, home string, width int) string {
	if len(run) == 1 {
		return toolLine(run[0], home, width)
	}
	line := cmdMark.Render("❯ ") + kit.StShared.Render(fmt.Sprintf("%d commands", len(run))) +
		kit.StHint.Render(" · "+toolNames(run))
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
	line := cmdMark.Render("⋯ ") + kit.StHint.Render(strings.Join(parts, " · "))
	return ansi.Truncate(line, width, "…")
}

// plural picks the format of a whole phrase, so a catalog can translate each form.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
