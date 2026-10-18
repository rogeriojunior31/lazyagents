package agents

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Agents", Keys: [][2]string{
			{"↑/↓ · j/k", "select agent"},
			{"shift+↑/↓ · ctrl+u/d", "scroll the detail"},
			{"g · G", "first · last"},
			{"d", "open local doctor report"},
			{"r", "refresh agent detection across tabs without network queries"},
		}},
		{Title: "Doctor report", Keys: [][2]string{
			{"←/→ · h/l", "select check section"},
			{"↑/↓ · pgup/pgdn", "scroll the report"},
			{"r", "rerun local checks"},
			{"n", "include usage/network and plugin checks"},
			{"enter", "open the related tab"},
			{"esc · q", "close report and cancel running checks"},
		}},
	}
}
