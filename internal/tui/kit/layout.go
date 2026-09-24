package kit

import (
	"strings"

	"charm.land/lipgloss/v2"
	"time"

	"charm.land/bubbles/v2/list"
)

// PaneID identifica o painel com foco no layout mestre/detalhe.
type PaneID int

const (
	PaneList PaneID = iota
	PaneDetail
)

// DetailScrollKeys são as teclas roteadas ao viewport do detalhe quando ele tem
// o foco; as demais teclas continuam agindo sobre a skill selecionada.
var DetailScrollKeys = map[string]bool{
	"up": true, "down": true, "j": true, "k": true,
	"pgup": true, "pgdown": true, "home": true, "end": true,
	"ctrl+u": true, "ctrl+d": true, "g": true, "G": true,
}

// ToastTTL é quanto um toast fica visível antes de sumir sozinho.
const ToastTTL = 4 * time.Second

// ListIndexAt converte uma linha da tela num índice absoluto da lista.
// Layout: borda superior do Panel (1) + linha em branco inicial da list (1) = 2
// linhas antes do primeiro item; depois 3 linhas por item (título + descrição +
// espaçamento).
// -1 = fora de qualquer item.
func ListIndexAt(l *list.Model, y int) int {
	row := y - 2
	if row < 0 {
		return -1
	}
	idx := l.Paginator.Page*l.Paginator.PerPage + row/3
	if idx >= len(l.VisibleItems()) {
		return -1
	}
	return idx
}

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

// RowAt converte o clique na linha y (relativa ao topo do painel) no índice
// da entrada, para listas desenhadas com ListRow num Panel de altura h: a
// faixa do título e uma linha em branco antes do primeiro item, depois 3
// linhas por item, na mesma janela (Window) que a renderização usa.
// -1 = fora de qualquer item.
func RowAt(y, cursor, total, h int) int {
	row := y - 2
	per := max(1, (h-2-1)/3)
	if row < 0 || row/3 >= per {
		return -1
	}
	start, end := Window(cursor, total, per)
	if idx := start + row/3; idx < end && row%3 < 2 {
		return idx
	}
	return -1
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
