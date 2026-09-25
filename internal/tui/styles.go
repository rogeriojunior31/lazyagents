package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Aliases for theme tokens; the palette lives in internal/tui/theme.
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
	badge   lipgloss.Style // "lazyagents" in an inverted block
	tagline lipgloss.Style // subtitle next to the badge
	status  lipgloss.Style // counters on the right of the header
	pill    lipgloss.Style // inactive tab
	pillOn  lipgloss.Style // active tab (filled)
	body    lipgloss.Style
}

func newStyles() styles {
	return styles{
		badge:   lipgloss.NewStyle().Foreground(theme.Bright).Bold(true).Padding(0, 2),
		tagline: lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 1),
		status:  lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 1),
		pill:    lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 2),
		pillOn:  lipgloss.NewStyle().Foreground(theme.OnAccent).Background(theme.Primary).Bold(true).Padding(0, 2),
		body:    lipgloss.NewStyle().Foreground(colorText).Padding(1, 2),
	}
}
