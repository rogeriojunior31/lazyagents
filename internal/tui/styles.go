package tui

import (
	"charm.land/lipgloss/v2"

	"lazyskills/internal/tui/theme"
)

// Aliases para os tokens do tema — a paleta canônica vive em internal/tui/theme.
var (
	colorPrimary = theme.Primary
	colorSubtle  = theme.Subtle
	colorBg      = theme.Bg
	colorText    = theme.Text
	colorBorder  = theme.Border
	colorOK      = theme.OK
	colorWarn    = theme.Warn
	colorErr     = theme.Err
)

type styles struct {
	title     lipgloss.Style
	tagline   lipgloss.Style
	tab       lipgloss.Style
	activeTab lipgloss.Style
	body      lipgloss.Style
	separator lipgloss.Style
}

func newStyles() styles {
	return styles{
		title:     lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Padding(0, 1),
		tagline:   lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 1),
		tab:       lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 2),
		activeTab: lipgloss.NewStyle().Foreground(colorBg).Background(colorPrimary).Bold(true).Padding(0, 2),
		body:      lipgloss.NewStyle().Foreground(colorText).Padding(1, 2),
		separator: lipgloss.NewStyle().Foreground(colorBorder),
	}
}
