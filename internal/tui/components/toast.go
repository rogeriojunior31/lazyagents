package components

import (
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Toast é a linha de feedback padrão da TUI: um selo colorido por tipo
// (✓ ok / ✗ erro) seguido da mensagem na mesma cor. Usado por Skills e Sessões
// para um visual idêntico. Operações em curso não passam por aqui — mostram o
// spinner com a mensagem em tom neutro.
var (
	toastOKBadge  = lipgloss.NewStyle().Foreground(theme.Bg).Background(theme.OK).Bold(true).Padding(0, 1)
	toastErrBadge = lipgloss.NewStyle().Foreground(theme.Bg).Background(theme.Err).Bold(true).Padding(0, 1)
	toastOKText   = lipgloss.NewStyle().Foreground(theme.OK)
	toastErrText  = lipgloss.NewStyle().Foreground(theme.Err)
)

func Toast(text string, isErr bool) string {
	if text == "" {
		return ""
	}
	if isErr {
		return toastErrBadge.Render("✗") + " " + toastErrText.Render(text)
	}
	return toastOKBadge.Render("✓") + " " + toastOKText.Render(text)
}
