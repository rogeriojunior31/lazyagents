package providers

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lists the Providers tab keys.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Providers", Keys: [][2]string{
			{"↑/↓ · j/k", "select profile"},
			{"←/→", "select agent (column)"},
			{"space", "preview apply; clear a unique managed match"},
			{"p", "preview/reapply the selected profile to the selected agent"},
			{"a", "preview changes and choose destination agents"},
			{"shift+↑/↓ · ctrl+u/d", "scroll the detail"},
			{"x", "clear the provider from all agents"},
			{"n", "choose a connection template, custom profile or agent defaults"},
			{"i", "import the selected agent into an editable library profile"},
			{"e", "edit the profile; an empty token keeps the saved one"},
			{"d", "delete the profile from the library"},
			{"c", "check the saved endpoint address without credentials"},
			{"m", "fetch an OpenAI-compatible catalog with profile credentials"},
			{"r", "refresh saved/live comparisons without writing"},
		}},
		{Title: "Connection templates and form", Keys: [][2]string{
			{"↑/↓ · j/k", "choose a starting point; enter opens the editable form"},
			{"tab · shift+tab (form)", "select a field and read its guidance"},
			{"shift+↑/↓ · ctrl+u/d", "scroll template details or focused-field guidance"},
			{"enter · ctrl+s (form)", "save once to the library; wait for completion before closing"},
			{"esc", "cancel; changed forms ask before discarding"},
		}},
		{Title: "Provider validation", Keys: [][2]string{
			{"↑/↓ · j/k", "select a destination and read its requirements"},
			{"space", "include/exclude an agent; incompatible selections block apply"},
			{"shift+↑/↓ · ctrl+u/d", "scroll destination details"},
			{"enter", "review masked field changes before confirmation"},
			{"esc", "cancel without writing"},
			{"ctrl+v (form)", "check the draft without saving or applying"},
		}},
		{Title: "Endpoint and model checks", Keys: [][2]string{
			{"ctrl+t (form)", "probe the draft endpoint without credentials"},
			{"ctrl+l (form)", "fetch the draft's OpenAI-compatible model catalog"},
			{"↑/↓ · type (catalog)", "select or filter a returned model"},
			{"shift+↑/↓ · ctrl+u/d", "scroll request evidence and errors"},
			{"enter (catalog)", "use the selected model in the form; save/apply stay separate"},
			{"esc", "cancel the request or return without changes"},
		}},
		{Title: "Bulk results", Keys: [][2]string{
			{"↑/↓ · pgup/pgdn", "scroll results; kept until dismissed"},
			{"r", "preview and confirm a retry of failed targets only"},
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
		{Name: "probe", Desc: "check the selected endpoint address without credentials", Msg: connectionActionMsg{}},
		{Name: "models", Desc: "fetch the selected profile's OpenAI-compatible catalog", Msg: connectionActionMsg{catalog: true}},
	}
}
