package skills

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Skills, agrupadas por assunto.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
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
			{"1-9", "toggle in agent N"},
			{"a", "enable in all"},
			{"x", "disable in all"},
		}},
		{Title: "Profiles & updates", Keys: [][2]string{
			{"p", "profiles"},
			{"u", "update this skill"},
			{"U", "check for updates"},
			{"b", "backups"},
		}},
		{Title: "List", Keys: [][2]string{
			{"shift+↑/↓", "scroll detail"},
			{"/", "filter"},
			{"r", "reload"},
		}},
	}
}
