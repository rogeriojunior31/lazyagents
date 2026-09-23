package sessions

import (
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
		{"", "sem projeto"},
		{"/", "sem projeto"},
		{"/tmp/lazyagents", "lazyagents"},
		{"/tmp/lazyagents/", "lazyagents"},
	}
	for _, c := range cases {
		if got := projectOf(agent.Session{CWD: c.cwd}); got != c.want {
			t.Errorf("projectOf(%q) = %q, quer %q", c.cwd, got, c.want)
		}
	}
}

func TestRenderTranscriptEmpty(t *testing.T) {
	out := renderTranscript(nil, 80, agent.Session{}, false).content
	if !strings.Contains(out, "vazio") {
		t.Errorf("transcript vazio deveria mostrar hint, veio:\n%s", out)
	}
}

// Mensagens seguidas do agente (texto e ferramentas) formam um turno só; os
// prompts ficam marcados para n/N.
func TestRenderTranscriptTurns(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "como faço X em Go?"},
		{Role: agent.RoleAssistant, Text: "Vou olhar."},
		{Role: agent.RoleTool, Text: "Bash · ls"},
		{Role: agent.RoleTool, Text: "Bash · go test"},
		{Role: agent.RoleTool, Text: "Read · a.go"},
		{Role: agent.RoleAssistant, Text: "Use o pacote `os`."},
		{Role: agent.RoleUser, Text: "obrigado"},
	}
	s := agent.Session{AgentID: "claude-code", AgentName: "Claude Code"}
	v := renderTranscript(entries, 60, s, false)
	out := ansi.Strip(v.content)

	if n := strings.Count(out, "◀ Claude Code"); n != 1 {
		t.Errorf("turnos do agente = %d, quer 1 (agrupados)\n%s", n, out)
	}
	if !strings.Contains(out, "▶ Você  #1") || !strings.Contains(out, "▶ Você  #2") {
		t.Errorf("prompts sem numeração:\n%s", out)
	}
	if !strings.Contains(out, "⚙ 3 chamadas · Bash ×2, Read") {
		t.Errorf("ferramentas não resumidas:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if len(v.prompts) != 2 || !strings.HasPrefix(lines[v.prompts[1]], "▶ Você") {
		t.Errorf("prompts = %v", v.prompts)
	}
	if v.stats != (transcriptStats{prompts: 2, replies: 1, tools: 3}) {
		t.Errorf("stats = %+v", v.stats)
	}
	if w := lipgloss.Width(v.content); w > 60 {
		t.Errorf("largura = %d, não deveria exceder 60", w)
	}

	// t: uma linha por chamada.
	out = ansi.Strip(renderTranscript(entries, 60, s, true).content)
	if !strings.Contains(out, "⚙ Bash  go test") || strings.Contains(out, "chamadas") {
		t.Errorf("ferramentas deveriam vir uma por linha:\n%s", out)
	}
}

func TestRenderTranscriptCapsWidth(t *testing.T) {
	// terminal muito largo: a conversa para em maxChatWidth.
	out := renderTranscript([]agent.Entry{{Role: agent.RoleUser, Text: strings.Repeat("palavra ", 80)}}, 500, agent.Session{}, false).content
	if w := lipgloss.Width(out); w > maxChatWidth {
		t.Errorf("largura = %d, não deveria exceder %d", w, maxChatWidth)
	}
}

func TestAliasInTitleAndFilter(t *testing.T) {
	s := agent.Session{AgentID: "claude-code", ID: "1", Title: "primeiro prompt", CWD: "/p/proj", Alias: "migração"}
	it := newSessionItem(s, "/h", false)
	if !strings.Contains(it.FilterValue(), "migração") || !strings.Contains(it.FilterValue(), "primeiro prompt") {
		t.Errorf("FilterValue = %q", it.FilterValue())
	}
	if !strings.Contains(it.Title(), "migração") {
		t.Errorf("Title = %q", it.Title())
	}
}
