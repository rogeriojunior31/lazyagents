package tui

import "charm.land/lipgloss/v2"

// Paleta Tokyo Night — mesma do vultrix-tui.
var (
	colorPrimary = lipgloss.Color("#7aa2f7") // azul — destaque, ativo
	colorSubtle  = lipgloss.Color("#565f89") // cinza-azulado — inativo, hints
	colorBg      = lipgloss.Color("#1a1b26") // fundo escuro — texto invertido
	colorText    = lipgloss.Color("#c0caf5") // texto principal
	colorBorder  = lipgloss.Color("#3b3d57") // bordas e separadores
	colorOK      = lipgloss.Color("#9ece6a") // verde — ativo/sucesso
	colorWarn    = lipgloss.Color("#e0af68") // âmbar — local/atenção
	colorErr     = lipgloss.Color("#f7768e") // vermelho — erro
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
