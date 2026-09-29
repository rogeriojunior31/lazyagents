package agent

import (
	"path/filepath"
	"sync"
)

// All returns every supported adapter in TUI display order, with an in-memory
// transcript index. home is injectable for tests.
func All(home string) []Adapter { return AllWithIndex(home, "") }

// AllWithIndex is All with the transcript index persisted at indexPath, so the
// next run only reads what agents appended since.
func AllWithIndex(home, indexPath string) []Adapter {
	idx := NewIndex(indexPath)
	claude, codex := NewClaude(home), NewCodex(home)
	claude.Index, codex.Index = idx, idx
	if indexPath != "" {
		// lazyagents' own data dir, next to the index: never the agent's files.
		claude.ProviderState = filepath.Join(filepath.Dir(indexPath), "claude-provider-state.json")
	}
	return []Adapter{
		claude,
		codex,
		NewGemini(home),
		NewOpenCode(home),
		NewClaudeDesktop(home),
		NewHermes(home),
		NewPi(home),
	}
}

// DetectAll runs Detect on every adapter in parallel (each may spend seconds on
// `--version`), keeping the order.
func DetectAll(adapters []Adapter) []Agent {
	agents := make([]Agent, len(adapters))
	var wg sync.WaitGroup
	for i, ad := range adapters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agents[i] = ad.Detect()
		}()
	}
	wg.Wait()
	return agents
}

// ByID returns the adapter with that ID, or nil.
func ByID(adapters []Adapter, id string) Adapter {
	for _, ad := range adapters {
		if ad.ID() == id {
			return ad
		}
	}
	return nil
}
