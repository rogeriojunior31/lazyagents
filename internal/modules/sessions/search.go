package sessions

import (
	"errors"
	"fmt"
	"strings"

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
// Busca case-insensitive por substring simples. Erros de transcript
// individuais não abortam a busca — agregados, mesmo padrão do List.
func SearchTranscripts(adapters []agent.Adapter, sessions []agent.Session, query string) ([]Match, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	var matches []Match
	var errs []error
	for _, s := range sessions {
		ad := agent.ByID(adapters, s.AgentID)
		if ad == nil {
			continue
		}
		entries, err := ad.Transcript(s)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.ID, err))
			continue
		}
		for _, e := range entries {
			if excerpt, ok := findExcerpt(e.Text, query); ok {
				matches = append(matches, Match{Session: s, Excerpt: excerpt})
				break // uma ocorrência já qualifica a sessão
			}
		}
	}
	return matches, errors.Join(errs...)
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
