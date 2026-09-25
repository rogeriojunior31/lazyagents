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

type transcriptStats struct{ prompts, replies, tools, thoughts int }

// chatColumn returns the centered column width (up to maxChatWidth) and its left indent.
func chatColumn(width int) (w, pad int) {
	w = max(20, min(width, maxChatWidth))
	return w, max(0, (width-w)/2)
}

// renderTranscript renders a chat in a centered column: user prompts on the
// right, agent turns on the left bordered in the agent color (see turnBlocks).
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
	bubble := func(c color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).Padding(0, 1)
	}

	var lines []string
	add := func(block string) {
		for _, ln := range strings.Split(block, "\n") {
			lines = append(lines, strings.Repeat(" ", pad)+ln)
		}
	}
	for i, t := range turns(entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		if t.user {
			v.stats.prompts++
			v.prompts = append(v.prompts, len(lines))
			// Sized to the text, up to 3/4 of the column, right-aligned.
			inner := max(10, w*3/4-4)
			body := strings.Join(turnBlocks(t, inner, o, &v.stats), "\n")
			body = lipgloss.NewStyle().MaxWidth(inner).Render(body)
			label := kit.StHint.Render(fmt.Sprintf("#%d  ", v.stats.prompts)) +
				lipgloss.NewStyle().Foreground(userColor).Bold(true).Render("You")
			add(lipgloss.PlaceHorizontal(w, lipgloss.Right, label))
			add(lipgloss.PlaceHorizontal(w, lipgloss.Right, bubble(userColor).Render(body)))
			continue
		}
		v.stats.replies++
		body := strings.Join(turnBlocks(t, w-4, o, &v.stats), "\n")
		add(lipgloss.NewStyle().Foreground(agentColor).Bold(true).Render(agentName))
		add(bubble(agentColor).Render(body))
	}
	v.content = strings.Join(lines, "\n")
	return v
}

// transcriptOpts is what the reader expands.
type transcriptOpts struct {
	tools    bool   // t: one command per line instead of a summary
	thinking bool   // r: full reasoning instead of its first line
	home     string // shortens command paths to ~
}

var (
	thinkStyle = lipgloss.NewStyle().Foreground(theme.Subtle).Italic(true)
	cmdMark    = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
)

// turnBlocks wraps a turn's blocks to width in the agent's order, with a blank
// line whenever the kind changes so reasoning, text and commands stay apart.
func turnBlocks(t turn, width int, o transcriptOpts, st *transcriptStats) []string {
	var out []string
	var run []string // pending consecutive commands
	last := ""       // role of the last emitted block
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
				lines[i] = toolLine(call, width)
			}
			emit(agent.RoleTool, strings.Join(lines, "\n"))
		} else {
			emit(agent.RoleTool, toolSummary(run, width))
		}
		run = nil
	}
	for _, e := range t.entries {
		switch e.Role {
		case agent.RoleTool:
			call := e.Text
			if o.home != "" {
				call = strings.ReplaceAll(call, o.home+"/", "~/")
			}
			run = append(run, call)
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

func toolLine(call string, width int) string {
	name, arg, _ := strings.Cut(call, " · ")
	line := cmdMark.Render("❯ ") + kit.StShared.Bold(true).Render(name) + "  " + kit.MdCode.Render(arg)
	return ansi.Truncate(line, width, "…")
}

// toolSummary: "❯ 4 commands · Bash ×3, Read"; a single command is shown whole.
func toolSummary(run []string, width int) string {
	if len(run) == 1 {
		return toolLine(run[0], width)
	}
	var order []string
	count := map[string]int{}
	for _, call := range run {
		name, _, _ := strings.Cut(call, " · ")
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
	line := cmdMark.Render("❯ ") + kit.StShared.Render(fmt.Sprintf("%d commands", len(run))) +
		kit.StHint.Render(" · "+strings.Join(parts, ", "))
	return ansi.Truncate(line, width, "…")
}
