package sessions

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// Match é uma sessão cujo transcript contém a busca, com um trecho de
// contexto ao redor da primeira ocorrência.
type Match struct {
	Session agent.Session
	Excerpt string
}

// excerptRadius é quantas runas de contexto entram de cada lado do trecho.
const excerptRadius = 40

// SearchTranscripts varre o transcript de cada sessão via Transcript() do
// adapter dono — sem conhecer paths/formatos, mesma regra 1 do agent.Adapter.
// Busca case-insensitive por substring simples. Adapter que sabe descartar
// sem decodificar (agent.TranscriptProber) pula quem certamente não contém;
// as sessões são lidas em paralelo e os resultados saem na ordem de entrada.
// Erros de transcript individuais não abortam a busca — agregados, mesmo
// padrão do List.
func SearchTranscripts(adapters []agent.Adapter, sessions []agent.Session, query string) ([]Match, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	type result struct {
		match *Match
		err   error
	}
	results := make([]result, len(sessions))
	work := make(chan int)
	var wg sync.WaitGroup
	for range max(1, min(runtime.NumCPU(), len(sessions))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				results[i].match, results[i].err = searchOne(adapters, sessions[i], query)
			}
		}()
	}
	for i := range sessions {
		work <- i
	}
	close(work)
	wg.Wait()

	var matches []Match
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		}
		if r.match != nil {
			matches = append(matches, *r.match)
		}
	}
	return matches, errors.Join(errs...)
}

// searchOne procura query numa sessão; nil = sem ocorrência.
func searchOne(adapters []agent.Adapter, s agent.Session, query string) (*Match, error) {
	ad := agent.ByID(adapters, s.AgentID)
	if ad == nil {
		return nil, nil
	}
	if p, ok := ad.(agent.TranscriptProber); ok && !p.MayContain(s, query) {
		return nil, nil
	}
	entries, err := ad.Transcript(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.ID, err)
	}
	for _, e := range entries {
		if excerpt, ok := findExcerpt(e.Text, query); ok {
			return &Match{Session: s, Excerpt: excerpt}, nil // uma ocorrência já qualifica a sessão
		}
	}
	return nil, nil
}

// findExcerpt procura query (case-insensitive) em text e devolve um trecho de
// ±excerptRadius runas ao redor da primeira ocorrência. ok=false = sem match.
func findExcerpt(text, query string) (string, bool) {
	runes := []rune(text)
	haystack := []rune(strings.ToLower(text))
	needle := []rune(strings.ToLower(query))
	idx := runeIndex(haystack, needle)
	if idx < 0 {
		return "", false
	}
	start := idx - excerptRadius
	if start < 0 {
		start = 0
	}
	end := idx + len(needle) + excerptRadius
	if end > len(runes) {
		end = len(runes)
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(runes) {
		suffix = "…"
	}
	return prefix + strings.TrimSpace(string(runes[start:end])) + suffix, true
}

// runeIndex é uma busca de substring ingênua em espaço de runas (evita
// mapear offsets de byte entre a versão original e a versão em minúsculas).
func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
