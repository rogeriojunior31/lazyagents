package components

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// KeycapStyle é o estilo único de "keycap" da TUI. Fonte única para os chips
// de tecla — modais (Keycap), rodapé de ajuda
// (help.Model, em app.go) e badges numéricos do detalhe (skills.go).
var KeycapStyle = lipgloss.NewStyle().Foreground(theme.Text).Background(theme.Sel).Padding(0, 1)

// Keycap renderiza uma tecla como chip, para dicas de teclado consistentes.
func Keycap(k string) string { return KeycapStyle.Render(k) }

// Panel is an open surface card: a title strip, padded content and a breathing
// row below. Focus is a small accent on the title, never a surrounding box.
// Width/Height include all padding; Height 0 grows with the content.
type Panel struct {
	Title   string
	Focused bool
	Width   int
	Height  int
	// Border overrides the title accent, used to identify agents and chat roles.
	Border color.Color
}

// ContentWidth discounts two columns of padding on either side.
func (p Panel) ContentWidth() int {
	w := p.Width - 4
	if w < 1 {
		return 1
	}
	return w
}

// ContentHeight discounts the title strip and the bottom breathing row.
// Só faz sentido com Height > 0; caso contrário devolve 0 (auto).
func (p Panel) ContentHeight() int {
	if p.Height <= 0 {
		return 0
	}
	h := p.Height - 2
	if h < 0 {
		return 0
	}
	return h
}

func (p Panel) Render(content string) string {
	borderColor := theme.Border
	if p.Focused {
		borderColor = theme.BorderFocus
	}
	titleColor := theme.Text
	if p.Focused {
		titleColor = theme.Primary
	}
	if p.Border != nil {
		borderColor, titleColor = p.Border, p.Border
	}
	bs := lipgloss.NewStyle().Foreground(borderColor).Background(theme.Surface)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Background(theme.Surface).Bold(true)

	innerW := p.Width - 2
	if innerW < 1 {
		innerW = 1
	}

	label := ansi.Truncate(p.Title, max(1, p.Width-4), "…")
	mark := "  "
	if p.Focused || p.Border != nil {
		mark = "▎ "
	}
	top := bs.Render(mark) + titleStyle.Width(max(1, p.Width-2)).Render(label)
	bottom := lipgloss.NewStyle().Background(theme.Surface).Width(p.Width).Render("")

	// Keep content padded and clipped without surrounding line art.
	cw := innerW - 2
	if cw < 1 {
		cw = 1
	}
	lines := strings.Split(content, "\n")
	if h := p.ContentHeight(); h > 0 {
		if len(lines) > h {
			lines = lines[:h]
		}
		for len(lines) < h {
			lines = append(lines, "")
		}
	}
	var b strings.Builder
	b.WriteString(top + "\n")
	for _, ln := range lines {
		ln = ansi.Truncate(ln, cw, "…")
		pad := cw - lipgloss.Width(ln)
		if pad < 0 {
			pad = 0
		}
		body := lipgloss.NewStyle().Foreground(theme.Text).Background(theme.Surface).
			Render("  " + ln + strings.Repeat(" ", pad) + "  ")
		b.WriteString(body + "\n")
	}
	b.WriteString(bottom)
	return theme.Paint(b.String(), theme.Text, theme.Surface)
}
