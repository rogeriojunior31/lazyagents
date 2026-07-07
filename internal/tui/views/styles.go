package views

import (
	"charm.land/lipgloss/v2"

	"lazyskills/internal/tui/theme"
)

// Estilos compartilhados pelas views. Cores vêm do tema central.
var (
	stTitle  = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	stText   = lipgloss.NewStyle().Foreground(theme.Text)
	stHint   = lipgloss.NewStyle().Foreground(theme.Subtle)
	stOn     = lipgloss.NewStyle().Foreground(theme.OK)
	stShared = lipgloss.NewStyle().Foreground(theme.Primary)
	stLocal  = lipgloss.NewStyle().Foreground(theme.Warn)
	stOff    = lipgloss.NewStyle().Foreground(theme.Subtle)
	stErr    = lipgloss.NewStyle().Foreground(theme.Err)
)
