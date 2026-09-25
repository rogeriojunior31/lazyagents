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
			t.Errorf("projectOf(%q) = %q, quer %q", c.cwd, got, c.want)
		}
	}
}

func TestRenderTranscriptEmpty(t *testing.T) {
	out := renderTranscript(nil, 80, agent.Session{}, transcriptOpts{}).content
	if !strings.Contains(out, "empty") {
		t.Errorf("transcript vazio deveria mostrar hint, veio:\n%s", out)
	}
}

// Mensagens seguidas do agente (texto e ferramentas) formam um turno só; os
// prompts ficam marcados para n/N.
func TestRenderTranscriptTurns(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "como faço X em Go?"},
		{Role: agent.RoleThinking, Text: "primeiro entender o erro\ndepois corrigir"},
		{Role: agent.RoleAssistant, Text: "Vou olhar."},
		{Role: agent.RoleTool, Text: "Bash · ls"},
		{Role: agent.RoleTool, Text: "Bash · go test"},
		{Role: agent.RoleTool, Text: "Read · a.go"},
		{Role: agent.RoleAssistant, Text: "Use o pacote `os`."},
		{Role: agent.RoleUser, Text: "obrigado"},
	}
	s := agent.Session{AgentID: "claude-code", AgentName: "Claude Code"}
	v := renderTranscript(entries, 60, s, transcriptOpts{})
	out := ansi.Strip(v.content)

	if n := strings.Count(out, "Claude Code"); n != 1 {
		t.Errorf("turnos do agente = %d, quer 1 (agrupados)\n%s", n, out)
	}
	if !strings.Contains(out, "#1  You") || !strings.Contains(out, "#2  You") {
		t.Errorf("prompts sem numeração:\n%s", out)
	}
	if !strings.Contains(out, "❯ 3 commands · Bash ×2, Read") {
		t.Errorf("comandos não resumidos:\n%s", out)
	}
	// Raciocínio recolhido: só a 1ª linha, e separado da fala por uma linha
	// em branco.
	if !strings.Contains(out, "💭 primeiro entender o erro…") || strings.Contains(out, "depois corrigir") {
		t.Errorf("raciocínio deveria vir recolhido:\n%s", out)
	}
	if !regexp.MustCompile(`erro…\s*│\n\s*│\s+│\n\s*│ Vou olhar`).MatchString(out) {
		t.Errorf("raciocínio e fala sem linha em branco entre eles:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if len(v.prompts) != 2 || !strings.HasSuffix(strings.TrimSpace(lines[v.prompts[1]]), "#2  You") {
		t.Errorf("prompts = %v", v.prompts)
	}
	if v.stats != (transcriptStats{prompts: 2, replies: 1, tools: 3, thoughts: 1}) {
		t.Errorf("stats = %+v", v.stats)
	}
	if w := lipgloss.Width(v.content); w > 60 {
		t.Errorf("largura = %d, não deveria exceder 60", w)
	}
	// Balão do usuário encostado à direita, do agente à esquerda.
	for _, ln := range lines {
		if strings.Contains(ln, "obrigado") && !strings.HasSuffix(ln, "│") {
			t.Errorf("balão do usuário não fecha na borda direita: %q", ln)
		}
		if strings.Contains(ln, "Vou olhar.") && !strings.HasPrefix(ln, "│") {
			t.Errorf("balão do agente não começa na esquerda: %q", ln)
		}
	}

	// t e r: um comando por linha e o raciocínio inteiro.
	out = ansi.Strip(renderTranscript(entries, 60, s, transcriptOpts{tools: true, thinking: true}).content)
	if !strings.Contains(out, "❯ Bash  go test") || strings.Contains(out, "commands ·") {
		t.Errorf("comandos deveriam vir um por linha:\n%s", out)
	}
	if !strings.Contains(out, "┆ primeiro entender o erro") || !strings.Contains(out, "┆ depois corrigir") {
		t.Errorf("raciocínio deveria vir inteiro:\n%s", out)
	}
}

func TestRenderTranscriptCapsWidthAndCenters(t *testing.T) {
	// Terminal muito largo: a conversa para em maxChatWidth, centralizada.
	entries := []agent.Entry{{Role: agent.RoleAssistant, Text: strings.Repeat("palavra ", 80)}}
	out := ansi.Strip(renderTranscript(entries, 300, agent.Session{}, transcriptOpts{}).content)
	_, pad := chatColumn(300)
	for _, ln := range strings.Split(out, "\n") {
		if ln == "" {
			continue
		}
		if !strings.HasPrefix(ln, strings.Repeat(" ", pad)) {
			t.Fatalf("linha sem o recuo de %d colunas: %q", pad, ln)
		}
		if w := lipgloss.Width(strings.TrimLeft(ln, " ")); w > maxChatWidth {
			t.Errorf("largura = %d, não deveria exceder %d", w, maxChatWidth)
		}
	}
}

func TestAliasInTitleAndFilter(t *testing.T) {
	s := agent.Session{AgentID: "claude-code", ID: "1", Title: "primeiro prompt", CWD: "/p/proj", Alias: "migração"}
	it := newSessionItem(s, false)
	if !strings.Contains(it.FilterValue(), "migração") || !strings.Contains(it.FilterValue(), "primeiro prompt") {
		t.Errorf("FilterValue = %q", it.FilterValue())
	}
	if !strings.Contains(it.Title(), "migração") {
		t.Errorf("Title = %q", it.Title())
	}
}
