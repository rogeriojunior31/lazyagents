package kit

import "testing"

func TestRowAt(t *testing.T) {
	// Painel de altura 14: título + linha em branco, depois 3 linhas por
	// item e cabem (14-3)/3 = 3 itens.
	cases := []struct{ y, cursor, total, want int }{
		{0, 0, 5, -1}, // faixa do título
		{1, 0, 5, -1}, // linha em branco
		{2, 0, 5, 0},  // título do 1º item
		{3, 0, 5, 0},  // descrição do 1º item
		{4, 0, 5, -1}, // espaço entre itens
		{5, 0, 5, 1},
		{8, 0, 5, 2},
		{11, 0, 5, -1}, // além da janela
		{2, 4, 5, 2},   // janela rolada até o fim: começa no item 2
		{5, 0, 1, -1},  // não há 2º item
	}
	for _, c := range cases {
		if got := RowAt(c.y, c.cursor, c.total, 14); got != c.want {
			t.Errorf("RowAt(y=%d, cursor=%d, total=%d) = %d, want %d", c.y, c.cursor, c.total, got, c.want)
		}
	}
}
