package providers

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lists the Providers tab keys.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Providers", Keys: [][2]string{
			{"↑/↓ · j/k", "select profile"},
			{"←/→", "select agent (column)"},
			{"space", "apply to the selected agent (again to clear)"},
			{"a", "validate and choose destination agents"},
			{"shift+↑/↓ · ctrl+u/d", "scroll the detail"},
			{"x", "clear the provider from all agents"},
			{"n", "choose a connection template, custom profile or agent defaults"},
			{"e", "edit the profile; an empty token keeps the saved one"},
			{"d", "delete the profile from the library"},
			{"r", "reload"},
		}},
		{Title: "Connection templates and form", Keys: [][2]string{
			{"↑/↓ · j/k", "choose a starting point; enter opens the editable form"},
			{"tab · shift+tab (form)", "select a field and read its guidance"},
			{"shift+↑/↓ · ctrl+u/d", "scroll template details or focused-field guidance"},
			{"enter · ctrl+s (form)", "save to the library; apply remains a separate action"},
			{"esc", "cancel the template/form without saving"},
		}},
		{Title: "Provider validation", Keys: [][2]string{
			{"↑/↓ · j/k", "select a destination and read its requirements"},
			{"space", "include/exclude an agent; incompatible selections block apply"},
			{"shift+↑/↓ · ctrl+u/d", "scroll destination details"},
			{"enter", "review selected compatible agents before confirmation"},
			{"esc", "cancel without writing"},
			{"ctrl+v (form)", "check the draft without saving or applying"},
		}},
		{Title: "Bulk results", Keys: [][2]string{
			{"↑/↓ · pgup/pgdn", "scroll results; kept until dismissed"},
			{"r", "retry only failed targets (confirm first)"},
			{"b", "open Recovery"},
			{"esc · enter", "dismiss the report and view refreshed state"},
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
		{Name: "new", Desc: "choose a connection template, custom profile or agent defaults", Msg: newProfileMsg{}},
		{Name: "clear", Desc: "clear the provider from all agents", Msg: clearAllMsg{}},
	}
}
