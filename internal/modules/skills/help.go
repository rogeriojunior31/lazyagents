package skills

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lists the Skills tab keys, grouped by subject.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Bulk results", Keys: [][2]string{
			{"↑/↓ · pgup/pgdn", "scroll results; kept until dismissed"},
			{"r", "retry only failed targets (confirm first)"},
			{"b", "open skill backups"},
			{"esc · enter", "dismiss the report and view refreshed state"},
		}},
		{Title: "Skills", Keys: [][2]string{
			{"enter", "read SKILL.md"},
			{"e", "edit in $EDITOR"},
			{"n", "new skill"},
			{"o", "adopt into library"},
			{"A", "adopt all local skills"},
			{"d", "remove (with backup)"},
			{"i", "install (GitHub/folder/zip)"},
			{"S", "search GitHub (registry)"},
		}},
		{Title: "Activation", Keys: [][2]string{
			{"←/→", "pick agent (column)"},
			{"space", "toggle in picked agent"},
			{"a", "enable in all"},
			{"x", "disable in all"},
		}},
		{Title: "Profiles & updates", Keys: [][2]string{
			{"p", "profiles: enter applies, s saves the matrix, d deletes"},
			{"u", "update this skill"},
			{"U", "check for updates"},
			{"b", "backups of selected skill (all when empty)"},
			{"B", "all skill backups; / filters by name or date"},
		}},
		{Title: "Inventory filters", Keys: [][2]string{
			{"↑/↓", "choose location, agent, metadata or updates"},
			{"←/→ · space", "cycle values; no files or network are read"},
			{"enter", "apply filters together with the text search"},
			{"r", "reset draft state filters; keep text search"},
			{"esc", "cancel without changing the active filters"},
		}},
		{Title: "List", Keys: [][2]string{
			{"shift+↑/↓", "scroll detail"},
			{"/", "search name, description, folder and source"},
			{"f", "filter location, enabled agent, metadata and cached update state"},
			{"F", "clear text and state filters; keep a visible selection"},
			{"esc", "clear text search; keep state filters"},
			{"r", "reload"},
		}},
	}
}

func (m Tab) Commands() []module.Command {
	return []module.Command{
		{Name: "backups", Desc: "browse all skill backups, including deleted skills", Msg: openBackupsMsg{}},
	}
}
