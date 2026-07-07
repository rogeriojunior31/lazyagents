package views

import (
	"charm.land/lipgloss/v2"

	"lazyskills/internal/tui/theme"
)

// asciiLogo é a arte ASCII do nome "lazyskills" em estilo figlet "big".
const asciiLogo = ` _                     _    _ _ _
| |                   | |  (_) | |
| | __ _ _____   _ ___| | ___| | |___
| |/ _` + "`" + ` |_  / | | / __| |/ / | | / __|
| | (_| |/ /| |_| \__ \   <| | | \__ \
|_|\__,_/___|\__, |___/_|\_\_|_|_|___/
              __/ |
             |___/`

var (
	splashLogoStyle = lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true)
	splashBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 4)
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
	logo := splashLogoStyle.Render(asciiLogo)
	ver := splashHintStyle.Render("v" + s.Version)
	tag := splashTagStyle.Render("skills e sessões para agentes de código AI")
	hints := splashHintStyle.Render("tab muda aba  ·  q sai  ·  ? ajuda")
	advance := splashHintStyle.Render("enter / espaço para avançar  ·  ou aguarde 2s…")

	inner := lipgloss.JoinVertical(lipgloss.Center,
		logo, "", ver, tag, "", hints, "", advance,
	)
	box := splashBoxStyle.Render(inner)

	if s.Width == 0 || s.Height == 0 {
		return box
	}
	return lipgloss.NewStyle().
		Width(s.Width).
		Height(s.Height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(box)
}
