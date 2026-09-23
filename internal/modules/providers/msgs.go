package providers

import "github.com/rogeriojunior31/lazyagents/internal/agent"

// loadedMsg traz a biblioteca de perfis e o estado de cada agente.
type loadedMsg struct {
	profiles []agent.ProviderProfile
	statuses []Status
	err      error
}

// doneMsg é o resultado de uma escrita (apply, clear, delete).
type doneMsg struct {
	text string
	err  bool
}

// clearAllMsg vem da paleta: providers clear.
type clearAllMsg struct{}

// profilesMsg é a leitura leve do boot: só os perfis, para o contador.
type profilesMsg struct{ profiles []agent.ProviderProfile }

// savedMsg é o resultado do formulário de perfil.
type savedMsg struct {
	name string
	err  error
}
