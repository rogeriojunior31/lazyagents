package skills

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lists the Skills tab keys, grouped by subject.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Update review and source progress", Keys: [][2]string{
			{"←/→", "in update review: inspect each skill and its recorded source"},
			{"↑/↓ · pgup/pgdn", "scroll changed files and bounded local-to-upstream diff"},
			{"enter", "review exact staged replacements, then confirm; back up first"},
			{"esc", "cancel clone, discovery or update; wait for cleanup"},
			{"u", "explicit single-item review may replace local edits; bulk preserves them"},
		}},
		{Title: "Source repositories", Keys: [][2]string{
			{"R", "open saved repositories; no automatic scans"},
			{"a · e", "in sources: add or edit URL, branch/tag and subfolder"},
			{"enter · s", "scan only the chosen source and pick entries to install"},
			{"d", "remove descriptor after confirmation; keep installed skills"},
			{"r", "reload descriptors without fetching repositories"},
			{"shift+↑/↓", "scroll source details, last scan and errors"},
			{"esc", "close sources or cancel the source form"},
		}},
		{Title: "Install picker", Keys: [][2]string{
			{"↑/↓ · space", "choose and mark entries or agent destinations"},
			{"a", "toggle all skills on the source step; all agents on destinations"},
			{"0 · x", "on destinations: library only, without activation links"},
			{"enter", "source: destinations; destinations: review before installing"},
			{"tab", "switch source entries and destinations"},
			{"pgup/pgdn", "scroll source warnings"},
			{"esc", "destinations: back to entries; entries: cancel and clean staging"},
		}},
		{Title: "Selected skill actions", Keys: [][2]string{
			{"m", "mark or unmark the current folder"},
			{"v", "replace selection with all visible rows (including off screen)"},
			{"V", "clear marked rows; hidden or removed rows are also unmarked"},
			{"c", "choose enable, disable, update or remove for marked rows"},
			{"↑/↓ · ←/→ · space", "in batch panel: choose action and agent destinations"},
			{"a · x", "in batch panel: select all agents or none"},
			{"enter · esc", "in batch panel: review frozen scope or cancel"},
		}},
		{Title: "Bulk results", Keys: [][2]string{
			{"↑/↓ · pgup/pgdn", "scroll results; kept until dismissed"},
			{"r", "retry failed targets when offered; updates need a fresh review"},
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
			{"u", "review changes before updating this skill"},
			{"U", "check all Git skills and review staged updates"},
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
		{Name: "sources", Desc: "manage saved skill repositories; scan only on request", Msg: openSourcesMsg{}},
	}
}
