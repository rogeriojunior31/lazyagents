package sessions

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

func TestProjectOf(t *testing.T) {
	cases := []struct {
		cwd  string
		want string
	}{
		{"", "no project"},
		{"/", "no project"},
		{"/tmp/lazyagents", "lazyagents"},
		{"/tmp/lazyagents/", "lazyagents"},
	}
	for _, c := range cases {
		if got := projectOf(agent.Session{CWD: c.cwd}); got != c.want {
			t.Errorf("projectOf(%q) = %q, want %q", c.cwd, got, c.want)
		}
	}
}

func TestRenderTranscriptEmpty(t *testing.T) {
	out := renderTranscript(nil, 80, agent.Session{}, transcriptOpts{}).content
	if !strings.Contains(out, "empty") {
		t.Errorf("empty transcript should show a hint, got:\n%s", out)
	}
}

// Consecutive agent messages form one turn; prompts are indexed for n/N.
func TestRenderTranscriptTurns(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "how do I do X in Go?"},
		{Role: agent.RoleThinking, Text: "first understand the error\nthen fix it"},
		{Role: agent.RoleAssistant, Text: "Let me look."},
		{Role: agent.RoleTool, Text: "Bash · ls"},
		{Role: agent.RoleTool, Text: "Bash · go test"},
		{Role: agent.RoleTool, Text: "Read · a.go"},
		{Role: agent.RoleAssistant, Text: "Use the `os` package."},
		{Role: agent.RoleUser, Text: "thanks"},
	}
	s := agent.Session{AgentID: "claude-code", AgentName: "Claude Code"}
	v := renderTranscript(entries, 60, s, transcriptOpts{})
	out := ansi.Strip(v.content)

	if n := strings.Count(out, "Claude Code"); n != 1 {
		t.Errorf("agent turns = %d, want 1 (grouped)\n%s", n, out)
	}
	if !strings.Contains(out, "#1  You") || !strings.Contains(out, "#2  You") {
		t.Errorf("prompts not numbered:\n%s", out)
	}
	if !strings.Contains(out, "❯ 3 commands · Bash ×2, Read") {
		t.Errorf("commands not summarized:\n%s", out)
	}
	// Collapsed reasoning: first line only, a blank line before the reply.
	if !strings.Contains(out, "💭 first understand the error…") || strings.Contains(out, "then fix it") {
		t.Errorf("reasoning should be collapsed:\n%s", out)
	}
	if !regexp.MustCompile(`error…\s*│\n\s*│\s+│\n\s*│ Let me look`).MatchString(out) {
		t.Errorf("no blank line between reasoning and reply:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if len(v.prompts) != 2 || !strings.HasSuffix(strings.TrimSpace(lines[v.prompts[1]]), "#2  You") {
		t.Errorf("prompts = %v", v.prompts)
	}
	if v.stats != (transcriptStats{prompts: 2, replies: 1, tools: 3, thoughts: 1}) {
		t.Errorf("stats = %+v", v.stats)
	}
	if w := lipgloss.Width(v.content); w > 60 {
		t.Errorf("width = %d, should not exceed 60", w)
	}
	// User bubble on the right, agent bubble on the left.
	for _, ln := range lines {
		if strings.Contains(ln, "thanks") && !strings.HasSuffix(ln, "│") {
			t.Errorf("user bubble does not end at the right edge: %q", ln)
		}
		if strings.Contains(ln, "Let me look.") && !strings.HasPrefix(ln, "│") {
			t.Errorf("agent bubble does not start at the left: %q", ln)
		}
	}

	// t and r: one command per line and the full reasoning.
	out = ansi.Strip(renderTranscript(entries, 60, s, transcriptOpts{tools: true, thinking: true}).content)
	if !strings.Contains(out, "❯ Bash  go test") || strings.Contains(out, "commands ·") {
		t.Errorf("commands should be one per line:\n%s", out)
	}
	if !strings.Contains(out, "┆ first understand the error") || !strings.Contains(out, "┆ then fix it") {
		t.Errorf("reasoning should be full:\n%s", out)
	}
}

func TestRenderTranscriptCapsWidthAndCenters(t *testing.T) {
	// Very wide terminal: the chat stops at maxChatWidth, centered.
	entries := []agent.Entry{{Role: agent.RoleAssistant, Text: strings.Repeat("word ", 80)}}
	out := ansi.Strip(renderTranscript(entries, 300, agent.Session{}, transcriptOpts{}).content)
	_, pad := chatColumn(300)
	for _, ln := range strings.Split(out, "\n") {
		if ln == "" {
			continue
		}
		if !strings.HasPrefix(ln, strings.Repeat(" ", pad)) {
			t.Fatalf("line without the %d-column indent: %q", pad, ln)
		}
		if w := lipgloss.Width(strings.TrimLeft(ln, " ")); w > maxChatWidth {
			t.Errorf("width = %d, should not exceed %d", w, maxChatWidth)
		}
	}
}

func TestAliasInTitleAndFilter(t *testing.T) {
	s := agent.Session{AgentID: "claude-code", ID: "1", Title: "first prompt", CWD: "/p/proj", Alias: "migration"}
	it := newSessionItem(s, false)
	if !strings.Contains(it.FilterValue(), "migration") || !strings.Contains(it.FilterValue(), "first prompt") {
		t.Errorf("FilterValue = %q", it.FilterValue())
	}
	if !strings.Contains(it.Title(), "migration") {
		t.Errorf("Title = %q", it.Title())
	}
}
