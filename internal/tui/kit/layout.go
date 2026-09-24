package kit

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// ToastTTL é quanto um toast fica visível antes de sumir sozinho.
const ToastTTL = 4 * time.Second

// Truncate corta s em max runas, com reticências.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// Window devolve a janela visível centrada no cursor.
func Window(cursor, total, size int) (int, int) {
	if size < 1 {
		size = 1
	}
	if total <= size {
		return 0, total
	}
	start := cursor - size/2
	if start < 0 {
		start = 0
	}
	if start+size > total {
		start = total - size
	}
	return start, start + size
}

// Wrap quebra s para caber em width colunas, com indent na frente de cada
// linha — para texto de card que não pode perder o fim (caminho, erro,
// endpoint), onde truncar esconderia justo a parte útil.
func Wrap(s string, width int, indent string) string {
	w := max(8, width-lipgloss.Width(indent))
	lines := strings.Split(lipgloss.NewStyle().Width(w).Render(s), "\n")
	for i, ln := range lines {
		lines[i] = indent + strings.TrimRight(ln, " ")
	}
	return strings.Join(lines, "\n")
}

// SideDetailWidth é a largura a partir da qual tabela e detalhe ficam lado a
// lado; abaixo dela o detalhe vira uma faixa sob a tabela.
const SideDetailWidth = 110

// Split é a divisão da área de uma aba entre tabela e detalhe.
type Split struct {
	Side             bool // detalhe à direita; senão, faixa embaixo
	ListW, ListH     int
	DetailW, DetailH int
}

// SplitDetail reparte width×height: lado a lado com a tabela em 3/5 quando há
// largura, senão a tabela em cima e uma faixa de 3–6 linhas para o detalhe.
func SplitDetail(width, height int) Split {
	width, height = max(1, width), max(1, height)
	if width >= SideDetailWidth {
		lw := width * 3 / 5
		return Split{Side: true, ListW: lw, ListH: height, DetailW: width - lw - 2, DetailH: height}
	}
	dh := min(max(3, height/3), 6, max(0, height-3))
	return Split{ListW: width, ListH: height - dh, DetailW: width, DetailH: dh}
}

// Frame encaixa o corpo em height linhas com o rodapé sempre na última: corpo
// curto ganha linhas em branco, corpo longo é cortado antes do rodapé.
func Frame(body, footer string, height int) string {
	foot := strings.Split(footer, "\n")
	if footer == "" {
		foot = nil
	}
	foot = foot[max(0, len(foot)-max(0, height)):]
	room := max(0, height-len(foot))
	lines := strings.Split(body, "\n")
	if len(lines) > room {
		lines = lines[:room]
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	return strings.Join(append(lines, foot...), "\n")
}
