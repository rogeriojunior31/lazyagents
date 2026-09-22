package agents

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help para a aba Agentes — somente leitura, sem teclas próprias.
func (m Agents) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Agentes", Keys: [][2]string{
			{"—", "somente leitura"},
		}},
	}
}
