package sessions

import (
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// maxChatWidth é a largura de leitura dos cards do transcript — mesmo com o
// terminal largo, a conversa não estica além disso.
const maxChatWidth = 96

// renderTranscript formata as mensagens como cards empilhados (estilo chat):
// um Panel por mensagem, papel no título, cor por papel e uma linha de respiro
// entre os turnos. width é a largura disponível no viewport.
func renderTranscript(entries []agent.Entry, width int) string {
	if len(entries) == 0 {
		return kit.StHint.Render("(transcript vazio ou em formato desconhecido)")
	}
	cardW := min(width, maxChatWidth)
	if cardW < 8 {
		cardW = 8
	}
	cards := make([]string, 0, len(entries))
	for _, e := range entries {
		p := components.Panel{Width: cardW}
		if e.Role == "user" {
			p.Title, p.Border = "▶ você", theme.Primary
		} else {
			p.Title, p.Border = "◀ agente", theme.Subtle
		}
		body := strings.TrimRight(kit.RenderMarkdown(e.Text, p.ContentWidth()), " \t\r\n")
		cards = append(cards, p.Render(body), "")
	}
	return strings.Join(cards, "\n")
}
