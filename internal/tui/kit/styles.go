// Package kit holds UI pieces shared by modules: theme styles, the one-line
// table with a side/bottom detail, light markdown and layout helpers. No I/O.
package kit

import (
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

var (
	StTitle  = lipgloss.NewStyle().Foreground(theme.Bright).Bold(true)
	StText   = lipgloss.NewStyle().Foreground(theme.Text)
	StHint   = lipgloss.NewStyle().Foreground(theme.Subtle)
	StOn     = lipgloss.NewStyle().Foreground(theme.OK)
	StShared = lipgloss.NewStyle().Foreground(theme.Primary)
	StLocal  = lipgloss.NewStyle().Foreground(theme.Warn)
	StOff    = lipgloss.NewStyle().Foreground(theme.Subtle)
	StErr    = lipgloss.NewStyle().Foreground(theme.Err)
	StWarn   = lipgloss.NewStyle().Foreground(theme.Warn)

	// diff (added / removed)
	StAdded   = lipgloss.NewStyle().Foreground(theme.Added)
	StRemoved = lipgloss.NewStyle().Foreground(theme.Removed)

	// detail cards (label/value)
	CardLabel = lipgloss.NewStyle().Foreground(theme.Subtle)
	CardValue = lipgloss.NewStyle().Foreground(theme.Text)
)
