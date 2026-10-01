package providers

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lists the Providers tab keys.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Providers", Keys: [][2]string{
			{"↑/↓ · j/k", "select profile"},
			{"←/→", "select agent (column)"},
			{"space", "apply to the selected agent (again to clear)"},
			{"a", "apply to all installed agents"},
			{"shift+↑/↓ · ctrl+u/d", "scroll the detail"},
			{"x", "clear the provider from all agents"},
			{"n", "new profile (form)"},
			{"e", "edit the profile; an empty token keeps the saved one"},
			{"d", "delete the profile from the library"},
			{"r", "reload"},
		}},
		{Title: "Create a profile", Keys: [][2]string{
			{"lazyagents provider add", "create a profile from the CLI"},
			{"--token -", "read the token from stdin"},
		}},
	}
}

// Commands exposes clearing in the palette (module.Commander).
func (m Tab) Commands() []module.Command {
	return []module.Command{
		{Name: "new", Desc: "create a provider profile", Msg: newProfileMsg{}},
		{Name: "clear", Desc: "clear the provider from all agents", Msg: clearAllMsg{}},
	}
}
