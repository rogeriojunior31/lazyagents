package hooks

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lists the Hooks tab keys.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Hooks", Keys: [][2]string{
			{"↑/↓ · j/k", "select hook"},
			{"←/→", "select agent (column)"},
			{"space", "install in the selected agent (again uninstalls)"},
			{"1-9", "install in agent N (again uninstalls)"},
			{"a", "install in every agent that fires the event"},
			{"enter", "pick the commands of the pack (space toggles, esc goes back)"},
			{"x", "uninstall from every agent"},
			{"d", "delete the hook from the library"},
			{"pgup/pgdn · ctrl+u/d", "read the detail and long commands, also while selecting"},
			{"shift+↑/↓", "scroll the detail one line"},
			{"v", "full-screen reader: ←→ switches command/scripts, e edits"},
			{"r", "reload"},
		}},
		{Title: "Create a hook", Keys: [][2]string{
			{"lazyagents hooks add", "create a hook from the CLI"},
			{"~/.local/share/lazyagents/hooks", "one JSON per hook, editable by hand"},
		}},
	}
}
