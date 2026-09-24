package agents

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help para a aba de diagnóstico.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Agentes", Keys: [][2]string{
			{"↑/↓ · j/k", "escolher agente"},
			{"shift+↑/↓ · ctrl+u/d", "rola o detalhe"},
			{"g · G", "primeiro · último"},
		}},
	}
}
