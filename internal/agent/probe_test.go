package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileMayContain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	// "Agulha" cruza a fronteira do primeiro bloco de leitura
	data := strings.Repeat("x", probeChunk-3) + `Agulha no palheiro`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		query string
		want  bool
	}{
		{"agulha", true},   // sem diferenciar maiúsculas, entre blocos
		{"PALHEIRO", true}, // fim do arquivo
		{"inexistente", false},
		{`com "aspas"`, true}, // JSON escaparia: a busca completa decide
		{"açúcar", true},      // não ASCII: idem
	}
	for _, tc := range cases {
		if got := fileMayContain(path, tc.query); got != tc.want {
			t.Errorf("fileMayContain(%q) = %v, quer %v", tc.query, got, tc.want)
		}
	}
	if !fileMayContain(filepath.Join(t.TempDir(), "nada"), "x") {
		t.Error("arquivo ilegível deve seguir para o Transcript, que reporta o erro")
	}
}
