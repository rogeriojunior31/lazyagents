package components

import (
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Toast is the standard feedback line: a ✓/✗ badge and the message in the same
// color. Operations in progress show a spinner instead.
var (
	toastOKBadge  = lipgloss.NewStyle().Foreground(theme.OnAccent).Background(theme.OK).Bold(true).Padding(0, 1)
	toastErrBadge = lipgloss.NewStyle().Foreground(theme.OnAccent).Background(theme.Err).Bold(true).Padding(0, 1)
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
