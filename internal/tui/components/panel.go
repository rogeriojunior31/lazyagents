package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"lazyskills/internal/tui/theme"
)

var keycapStyle = lipgloss.NewStyle().Foreground(theme.Bg).Background(theme.Border).Bold(true).Padding(0, 1)

// Keycap renderiza uma tecla como chip (texto escuro sobre fundo de borda),
// para dicas de teclado consistentes nos modais.
func Keycap(k string) string { return keycapStyle.Render(k) }

// Panel é o painel emoldurado padrão da TUI (estilo yazi/lazygit): borda
// arredondada com o título embutido na aresta superior e cor de borda variável
// conforme o foco. Largura/altura são o total EM COLUNAS/LINHAS incluindo a
// borda; Height 0 = auto (cresce com o conteúdo).
type Panel struct {
	Title   string
	Focused bool
	Width   int
	Height  int
}

// ContentWidth é a largura útil para o conteúdo (descontadas borda + padding
// lateral de 1). Callers usam isto para quebrar/truncar o conteúdo antes.
func (p Panel) ContentWidth() int {
	w := p.Width - 4
	if w < 1 {
		return 1
	}
	return w
}

// ContentHeight é a altura útil para o conteúdo (descontadas as duas bordas).
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
	bs := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(theme.Subtle).Bold(true)
	if p.Focused {
		titleStyle = titleStyle.Foreground(theme.Primary)
	}

	innerW := p.Width - 2 // colunas entre as duas barras verticais
	if innerW < 1 {
		innerW = 1
	}

	// Aresta superior: ╭─ Título ─────╮ (título só se couber).
	var top string
	if p.Title != "" && innerW >= 6 {
		t := p.Title
		if lipgloss.Width(t)+4 > innerW {
			t = ansi.Truncate(t, innerW-4, "…")
		}
		fill := innerW - lipgloss.Width(t) - 3 // "─ " + título + " "
		if fill < 0 {
			fill = 0
		}
		top = bs.Render("╭─ ") + titleStyle.Render(t) + bs.Render(" "+strings.Repeat("─", fill)+"╮")
	} else {
		top = bs.Render("╭" + strings.Repeat("─", innerW) + "╮")
	}
	bottom := bs.Render("╰" + strings.Repeat("─", innerW) + "╯")

	// Corpo: cada linha vira │ <conteúdo padded> │, com padding lateral de 1.
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
	bar := bs.Render("│")
	var b strings.Builder
	b.WriteString(top + "\n")
	for _, ln := range lines {
		ln = ansi.Truncate(ln, cw, "…")
		pad := cw - lipgloss.Width(ln)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(bar + " " + ln + strings.Repeat(" ", pad) + " " + bar + "\n")
	}
	b.WriteString(bottom)
	return b.String()
}
