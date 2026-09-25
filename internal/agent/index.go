package agent

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Índice incremental dos transcripts JSONL.
//
// Transcript de agente só cresce: cada linha nova é anexada ao fim. O índice
// guarda, por arquivo, até onde já leu e o que extraiu (prévia, soma de
// tokens, eventos de uso, limites); na próxima consulta, arquivo igual custa
// um stat, e arquivo que cresceu é lido só a partir do offset. Arquivo
// reescrito (encolheu ou mudou o começo) é relido do zero.
//
// É cache: apagar o arquivo só custa reler tudo uma vez. Nunca guarda texto
// de conversa além do título, que já é exibido na lista.

// indexVersion muda quando indexEntry muda de forma: índice antigo é descartado.
const indexVersion = 2

// headLen é quantos bytes do começo do arquivo identificam o conteúdo.
const headLen = 256

// usageSlot agrupa as respostas em faixas de 15 minutos: todo fuso real é
// múltiplo de 15 min, então uma faixa nunca cruza a meia-noite local (o dia
// continua exato), e o bloco de 5h começa na data exata da primeira resposta.
const usageSlot = 15 * time.Minute

// usageBucket soma as respostas de uma faixa, de um modelo e de uma pasta.
// Compacto de propósito: o índice guarda milhares deles.
type usageBucket struct {
	First                          int64  // unix nano da primeira resposta da faixa (0 = linha sem data)
	M                              int    // índice do modelo em indexEntry.Models
	CWD                            string // "" = igual a indexEntry.CWD; noCWD = a linha não tinha
	In, Out, CacheRead, CacheWrite int
	N                              int // respostas somadas
}

// noCWD marca resposta sem pasta na linha: vale a da sessão. Caminho nunca
// contém NUL, então não colide com pasta real.
const noCWD = "\x00"

func slotOf(first int64) int64 {
	if first == 0 {
		return -1
	}
	return first / int64(usageSlot)
}

// indexEntry é o que o índice sabe de um arquivo.
type indexEntry struct {
	Size, ModTime, Offset int64
	HeadLen               int
	Head                  uint64

	// prévia: pasta da sessão (primeira vista), primeiro prompt, título
	CWD, FirstPrompt, AITitle string
	// soma de tokens (Claude)
	Usage    Usage
	HasUsage bool
	// respostas agrupadas; Legacy são os token_count antigos do Codex, só
	// usados quando não há nenhum token_usage_record
	Events, Legacy []usageBucket
	Models         []string
	// contexto corrente do rollout (Codex): modelo e pasta herdados
	Model, CtxCWD string
	// últimos limites registrados (Codex)
	Rate   *codexRateLimits
	RateAt int64
}

// addEvent soma uma resposta na faixa dela, em list (Events ou Legacy).
func (e *indexEntry) addEvent(list *[]usageBucket, ts time.Time, model, cwd string, u Usage) {
	first := int64(0)
	if !ts.IsZero() {
		first = ts.UnixNano()
	}
	switch cwd {
	case "":
		cwd = noCWD
	case e.CWD:
		cwd = ""
	}
	m := slices.Index(e.Models, model)
	if m < 0 {
		e.Models = append(e.Models, model)
		m = len(e.Models) - 1
	}
	slot := slotOf(first)
	// respostas chegam em ordem: a faixa, se existe, está no fim
	for i := len(*list) - 1; i >= 0 && slotOf((*list)[i].First) == slot; i-- {
		b := &(*list)[i]
		if b.M == m && b.CWD == cwd {
			b.In += u.Input
			b.Out += u.Output
			b.CacheRead += u.CacheRead
			b.CacheWrite += u.CacheWrite
			b.N++
			return
		}
	}
	*list = append(*list, usageBucket{First: first, M: m, CWD: cwd,
		In: u.Input, Out: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, N: 1})
}

// events converte as faixas em UsageEvent; sem data ou sem pasta, valem as
// da sessão.
func (e indexEntry) events(list []usageBucket, s Session) []UsageEvent {
	out := make([]UsageEvent, 0, len(list))
	for _, b := range list {
		ts := s.MTime
		if b.First != 0 {
			ts = time.Unix(0, b.First)
		}
		cwd := b.CWD
		switch cwd {
		case "":
			cwd = e.CWD
		case noCWD:
			cwd = s.CWD
		}
		model := ""
		if b.M < len(e.Models) {
			model = e.Models[b.M]
		}
		out = append(out, UsageEvent{Time: ts, Model: model, CWD: cwd, N: b.N, Usage: Usage{
			Input: b.In, Output: b.Out, CacheRead: b.CacheRead, CacheWrite: b.CacheWrite, Model: model}})
	}
	return out
}

// lineScanner extrai de uma linha o que interessa ao adapter.
type lineScanner func(e *indexEntry, line []byte)

// Index é o índice compartilhado pelos adapters. path "" = só em memória.
type Index struct {
	path    string
	mu      sync.Mutex
	entries map[string]*indexEntry
	dirty   bool
}

// NewIndex abre o índice gravado em path (ausente, ilegível ou de outra
// versão = vazio).
func NewIndex(path string) *Index {
	x := &Index{path: path, entries: map[string]*indexEntry{}}
	if path == "" {
		return x
	}
	f, err := os.Open(path)
	if err != nil {
		return x
	}
	defer f.Close()
	var disk struct {
		Version int
		Entries map[string]*indexEntry
	}
	if gob.NewDecoder(f).Decode(&disk) == nil && disk.Version == indexVersion && disk.Entries != nil {
		x.entries = disk.Entries
	}
	return x
}

// save grava o índice se algo mudou. Falha não é erro de usuário: é cache.
func (x *Index) save() {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.path == "" || !x.dirty {
		return
	}
	var buf bytes.Buffer
	disk := struct {
		Version int
		Entries map[string]*indexEntry
	}{indexVersion, x.entries}
	if gob.NewEncoder(&buf).Encode(disk) != nil {
		return
	}
	if fsutil.WriteAtomic(x.path, buf.Bytes(), 0o600) == nil {
		x.dirty = false
	}
}

// retain esquece os arquivos sob prefix que não estão em keep (sessões
// apagadas), para o índice não crescer para sempre.
func (x *Index) retain(prefix string, keep map[string]bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for p := range x.entries {
		if strings.HasPrefix(p, prefix) && !keep[p] {
			delete(x.entries, p)
			x.dirty = true
		}
	}
}

// get devolve a entrada de path como está no índice.
func (x *Index) get(path string) (indexEntry, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	e, ok := x.entries[path]
	if !ok {
		return indexEntry{}, false
	}
	return *e, true
}

// refresh põe a entrada de path em dia com o arquivo e a devolve.
func (x *Index) refresh(path string, scan lineScanner) (indexEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return indexEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	old, known := x.get(path)
	if known && old.Size == info.Size() && old.ModTime == info.ModTime().UnixNano() {
		return old, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return indexEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()
	e := indexEntry{}
	if known && info.Size() >= old.Offset && old.HeadLen > 0 && hashAt(f, old.HeadLen) == old.Head {
		// só cresceu: continua de onde parou, sem mexer no que já foi lido
		e = old
		e.Events, e.Legacy, e.Models = slices.Clone(old.Events), slices.Clone(old.Legacy), slices.Clone(old.Models)
	}
	e.HeadLen = int(min(headLen, info.Size()))
	e.Head = hashAt(f, e.HeadLen)
	if _, err := f.Seek(e.Offset, io.SeekStart); err != nil {
		return indexEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	e.Offset += scanLines(f, func(line []byte) { scan(&e, line) })
	e.Size, e.ModTime = info.Size(), info.ModTime().UnixNano()

	x.mu.Lock()
	x.entries[path] = &e
	x.dirty = true
	x.mu.Unlock()
	return e, nil
}

// refreshAll põe em dia vários arquivos em paralelo (a primeira carga lê o
// histórico inteiro; com um worker por CPU ela não é serial).
func (x *Index) refreshAll(paths []string, scan lineScanner) {
	work := make(chan string)
	var wg sync.WaitGroup
	for range max(1, min(runtime.NumCPU(), len(paths))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range work {
				_, _ = x.refresh(p, scan) // ilegível: a sessão sai sem prévia
			}
		}()
	}
	for _, p := range paths {
		work <- p
	}
	close(work)
	wg.Wait()
}

// hashAt é o hash dos primeiros n bytes do arquivo (0 se não der para ler).
func hashAt(f *os.File, n int) uint64 {
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, 0); err != nil && !errors.Is(err, io.EOF) {
		return 0
	}
	h := fnv.New64a()
	h.Write(buf)
	return h.Sum64()
}

// scanLines entrega cada linha completa de r e devolve quantos bytes
// consumiu. Linha final sem quebra (o agente ainda escrevendo) só entra se
// já for JSON válido; senão fica para a próxima leitura. Linha acima de
// maxLineBuf encerra a leitura ali, como antes do índice.
func scanLines(r io.Reader, fn func([]byte)) int64 {
	var consumed, pending int64
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			pending = int64(i + 1)
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 && json.Valid(data) {
			pending = int64(len(data))
			return len(data), data, bufio.ErrFinalToken
		}
		if atEOF {
			return 0, nil, bufio.ErrFinalToken
		}
		return 0, nil, nil
	})
	for sc.Scan() {
		consumed += pending
		pending = 0
		if line := sc.Bytes(); len(line) > 0 {
			fn(line)
		}
	}
	return consumed
}
