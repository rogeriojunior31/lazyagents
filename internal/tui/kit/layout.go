package kit

import (
	"time"

	"charm.land/bubbles/v2/list"
)

// PaneID identifica o painel com foco no layout mestre/detalhe (M7.4).
type PaneID int

const (
	PaneList PaneID = iota
	PaneDetail
)

// DetailScrollKeys são as teclas roteadas ao viewport do detalhe quando ele tem
// o foco (M7.4); as demais teclas continuam agindo sobre a skill selecionada.
var DetailScrollKeys = map[string]bool{
	"up": true, "down": true, "j": true, "k": true,
	"pgup": true, "pgdown": true, "home": true, "end": true,
	"ctrl+u": true, "ctrl+d": true, "g": true, "G": true,
}

// ToastTTL é quanto um toast fica visível antes de sumir sozinho (M7.3).
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
