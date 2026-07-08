package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
)

func TestRenderTranscriptEmpty(t *testing.T) {
	out := renderTranscript(nil, 80)
	if !strings.Contains(out, "vazio") {
		t.Errorf("transcript vazio deveria mostrar hint, veio:\n%s", out)
	}
}

func TestRenderTranscriptCards(t *testing.T) {
	entries := []agent.Entry{
		{Role: "user", Text: "como faço X em Go?"},
		{Role: "assistant", Text: "Use o pacote `os`."},
		{Role: "user", Text: "obrigado"},
	}
	out := renderTranscript(entries, 60)

	// um card (borda superior ╭) por mensagem
	if n := strings.Count(out, "╭"); n != len(entries) {
		t.Errorf("cards = %d, quer %d\n%s", n, len(entries), out)
	}
	// papéis visíveis
	for _, want := range []string{"você", "agente"} {
		if !strings.Contains(out, want) {
			t.Errorf("papel %q não aparece:\n%s", want, out)
		}
	}
	// largura de leitura respeitada (cardW = min(width, 96) = 60)
	if w := lipgloss.Width(out); w > 60 {
		t.Errorf("largura = %d, não deveria exceder 60", w)
	}
}

func TestRenderTranscriptCapsWidth(t *testing.T) {
	// terminal muito largo: os cards param em maxChatWidth.
	out := renderTranscript([]agent.Entry{{Role: "user", Text: "oi"}}, 500)
	if w := lipgloss.Width(out); w > maxChatWidth {
		t.Errorf("largura = %d, não deveria exceder %d", w, maxChatWidth)
	}
}
