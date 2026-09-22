package components

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

var (
	splashLogoStyle = lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true)
	splashBoxStyle = lipgloss.NewStyle().
			Background(theme.Surface).
			Padding(2, 4)
	splashTagStyle  = lipgloss.NewStyle().Foreground(theme.Text)
	splashHintStyle = lipgloss.NewStyle().Foreground(theme.Subtle)
)

// Splash é a tela de boas-vindas. Não tem Update próprio — o root model
// (app.go) controla a transição para o estado principal.
type Splash struct {
	Version       string
	Width, Height int
}

func NewSplash(version string) Splash { return Splash{Version: version} }

func (s Splash) Resize(w, h int) Splash {
	s.Width, s.Height = w, h
	return s
}

func (s Splash) View() string {
	logo := splashLogoStyle.Render("  ▗▄▖\n ▐▛ ▜▌\n ▝▜▄▛▘") + "\n\n" +
		lipgloss.NewStyle().Foreground(theme.Bright).Bold(true).Render("l a z y a g e n t s")
	if s.Width > 0 && s.Width < 72 {
		logo = splashLogoStyle.Render("◈ lazyagents")
	}
	ver := splashHintStyle.Render("v" + s.Version)
	tag := splashTagStyle.Render("Um espaço para todos os seus agentes.")
	hints := splashHintStyle.Render("tab muda aba  ·  q sai  ·  ? ajuda")
	advance := splashHintStyle.Render("enter / espaço para avançar  ·  ou aguarde 2s…")

	inner := lipgloss.JoinVertical(lipgloss.Center,
		logo, "", ver, tag, "", hints, "", advance,
	)
	box := theme.Paint(splashBoxStyle.Render(inner), theme.Text, theme.Surface)

	if s.Width == 0 || s.Height == 0 {
		return box
	}
	if s.Width < 72 || s.Height < 22 {
		box = lipgloss.JoinVertical(lipgloss.Center, splashLogoStyle.Render("◈ lazyagents"), ver, "",
			ansi.Truncate(tag, s.Width, "…"), "", ansi.Truncate(advance, s.Width, "…"))
	}
	return theme.Paint(lipgloss.NewStyle().Background(theme.Bg).
		Width(s.Width).
		Height(s.Height).
		MaxWidth(s.Width).MaxHeight(s.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(box), theme.Text, theme.Bg)
}
