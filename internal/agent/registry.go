package agent

import "sync"

// All devolve os adapters de todos os agentes suportados, na ordem fixa de
// exibição da TUI. home é injetável para testes.
func All(home string) []Adapter {
	return []Adapter{
		NewClaude(home),
		NewCodex(home),
		NewGemini(home),
		NewOpenCode(home),
		NewClaudeDesktop(home),
		NewHermes(home),
	}
}

// DetectAll roda Detect em todos os adapters em paralelo (cada Detect pode
// gastar segundos rodando `--version`), preservando a ordem.
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

// ByID encontra o adapter de um agente pelo ID (nil se não existir).
func ByID(adapters []Adapter, id string) Adapter {
	for _, ad := range adapters {
		if ad.ID() == id {
			return ad
		}
	}
	return nil
}
