package views

import "charm.land/lipgloss/v2"

// Estilos compartilhados pelas views (paleta Tokyo Night).
var (
	stTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
	stText   = lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5"))
	stHint   = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	stOn     = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	stShared = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	stLocal  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	stOff    = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	stErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
)
