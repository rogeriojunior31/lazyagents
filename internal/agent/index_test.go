package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	userLine = `{"type":"user","cwd":"/p","message":{"role":"user","content":"primeiro pedido"}}`
	respLine = `{"type":"assistant","timestamp":"2026-09-22T10:00:00Z","message":{"model":"claude-opus-4","usage":{"input_tokens":10,"output_tokens":1}}}`
)

func appendTo(t *testing.T, path, data string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(data); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

// counter conta as linhas que o índice entrega ao adapter.
func counter(n *int) lineScanner {
	return func(e *indexEntry, line []byte) {
		*n++
		claudeIndexLine(e, line)
	}
}

func TestIndexReadsOnlyAppendedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	appendTo(t, path, userLine+"\n"+respLine+"\n")
	x := NewIndex("")
	lines := 0
	e, err := x.refresh(path, counter(&lines))
	if err != nil || lines != 2 || e.Usage.Input != 10 || e.FirstPrompt != "primeiro pedido" || e.CWD != "/p" {
		t.Fatalf("1ª leitura: %d linhas, %+v, %v", lines, e, err)
	}
	if _, _ = x.refresh(path, counter(&lines)); lines != 2 {
		t.Errorf("arquivo igual foi relido (%d linhas)", lines)
	}
	appendTo(t, path, respLine+"\n")
	e, _ = x.refresh(path, counter(&lines))
	if lines != 3 || e.Usage.Input != 20 || e.FirstPrompt != "primeiro pedido" {
		t.Errorf("crescimento: %d linhas lidas, %+v", lines, e)
	}
	// mesma faixa de 15 min: uma faixa com duas respostas
	if len(e.Events) != 1 || e.Events[0].N != 2 {
		t.Errorf("faixas = %+v", e.Events)
	}
}

// Linha final sem quebra (agente escrevendo) espera a próxima leitura; se já
// é JSON completo, entra.
func TestIndexPartialLastLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	appendTo(t, path, respLine+"\n"+respLine[:30])
	x := NewIndex("")
	e, _ := x.refresh(path, claudeIndexLine)
	if e.Usage.Input != 10 || e.Offset != int64(len(respLine)+1) {
		t.Fatalf("linha pela metade entrou: %+v", e)
	}
	appendTo(t, path, respLine[30:])
	if e, _ = x.refresh(path, claudeIndexLine); e.Usage.Input != 20 {
		t.Errorf("linha completada (sem quebra, JSON válido) não entrou: %+v", e)
	}
}

func TestIndexRewrittenFileIsReread(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	appendTo(t, path, userLine+"\n"+respLine+"\n"+respLine+"\n")
	x := NewIndex("")
	x.refresh(path, claudeIndexLine)
	// encolheu
	if err := os.WriteFile(path, []byte(respLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, _ := x.refresh(path, claudeIndexLine)
	if e.Usage.Input != 10 || e.FirstPrompt != "" {
		t.Errorf("arquivo menor não foi relido: %+v", e)
	}
	// mesmo tamanho ou maior, mas começo diferente
	other := `{"type":"user","cwd":"/q","message":{"role":"user","content":"outro"}}`
	if err := os.WriteFile(path, []byte(other+"\n"+respLine+"\n"+respLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if e, _ = x.refresh(path, claudeIndexLine); e.CWD != "/q" || e.Usage.Input != 20 {
		t.Errorf("arquivo reescrito não foi relido: %+v", e)
	}
}

func TestIndexPersistsAndRetains(t *testing.T) {
	dir := t.TempDir()
	idx := filepath.Join(dir, "idx.gob")
	a, b := filepath.Join(dir, "a.jsonl"), filepath.Join(dir, "b.jsonl")
	appendTo(t, a, userLine+"\n"+respLine+"\n")
	appendTo(t, b, respLine+"\n")
	x := NewIndex(idx)
	x.refreshAll([]string{a, b}, claudeIndexLine)
	x.retain(dir+string(filepath.Separator), map[string]bool{a: true})
	x.save()

	y := NewIndex(idx)
	if _, ok := y.get(b); ok {
		t.Error("retain não esqueceu o arquivo removido")
	}
	lines := 0
	if e, _ := y.refresh(a, counter(&lines)); lines != 0 || e.Usage.Input != 10 {
		t.Errorf("índice do disco não evitou a releitura: %d linhas, %+v", lines, e)
	}
	// índice corrompido é descartado, sem erro
	if err := os.WriteFile(idx, []byte("lixo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if z := NewIndex(idx); len(z.entries) != 0 {
		t.Errorf("índice corrompido virou %d entradas", len(z.entries))
	}
}

func TestIndexBucketsKeepDayAndCWD(t *testing.T) {
	var e indexEntry
	e.CWD = "/p"
	t0 := time.Date(2026, 9, 22, 10, 1, 0, 0, time.UTC)
	e.addEvent(&e.Events, t0, "m", "/p", Usage{Input: 1})
	e.addEvent(&e.Events, t0.Add(5*time.Minute), "m", "/p", Usage{Input: 1})  // mesma faixa
	e.addEvent(&e.Events, t0.Add(20*time.Minute), "m", "/p", Usage{Input: 1}) // outra faixa
	e.addEvent(&e.Events, t0.Add(21*time.Minute), "m", "/outra", Usage{Input: 1})
	e.addEvent(&e.Events, time.Time{}, "m", "", Usage{Input: 1}) // sem data nem pasta
	s := Session{CWD: "/sessao", MTime: t0.Add(time.Hour)}
	evs := e.events(e.Events, s)
	if len(evs) != 4 || evs[0].N != 2 || !evs[0].Time.Equal(t0) || evs[0].CWD != "/p" || evs[2].CWD != "/outra" {
		t.Fatalf("eventos = %+v", evs)
	}
	if last := evs[3]; !last.Time.Equal(s.MTime) || last.CWD != "/sessao" || last.Model != "m" {
		t.Errorf("sem data/pasta deve cair na sessão: %+v", last)
	}
}
