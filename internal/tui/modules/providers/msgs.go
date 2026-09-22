package providers

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/provider"
)

// loadedMsg traz a biblioteca de perfis e o estado de cada agente.
type loadedMsg struct {
	profiles []agent.ProviderProfile
	statuses []provider.Status
	err      error
}

// doneMsg é o resultado de uma escrita (apply, clear, delete).
type doneMsg struct {
	text string
	err  bool
}

// clearAllMsg vem da paleta: providers clear.
type clearAllMsg struct{}
