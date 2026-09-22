// Package kit reúne as peças de UI compartilhadas pelos módulos da TUI:
// estilos derivados do theme, delegate de lista, markdown leve, foco de painel
// e helpers de layout. Nada aqui faz I/O.
package kit

import (
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Estilos compartilhados pelos módulos. Cores vêm do tema central.
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

	// cards de detalhe (rótulo/valor)
	CardLabel = lipgloss.NewStyle().Foreground(theme.Subtle)
	CardValue = lipgloss.NewStyle().Foreground(theme.Text)
)
