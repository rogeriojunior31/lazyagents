package agents

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Agents", Keys: [][2]string{
			{"↑/↓ · j/k", "select agent"},
			{"shift+↑/↓ · ctrl+u/d", "scroll the detail"},
			{"g · G", "first · last"},
		}},
	}
}
