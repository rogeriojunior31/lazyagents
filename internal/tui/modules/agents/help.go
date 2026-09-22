package agents

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help para a aba de diagnóstico.
func (m Agents) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Agentes", Keys: [][2]string{
			{"↑/↓ · j/k", "rolar agentes"},
			{"g / home", "voltar ao início"},
		}},
	}
}
