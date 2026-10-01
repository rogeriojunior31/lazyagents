package sessions

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Sessions", Keys: [][2]string{
			{"enter", "resume"},
			{"v", "read the transcript"},
			{"R", "resume in another folder"},
			{"c", "show the command"},
			{"m", "alias (empty removes it)"},
			{"d", "delete (with backup)"},
		}},
		{Title: "List", Keys: [][2]string{
			{"shift+↑/↓", "scroll the detail"},
			{"pgup/pgdn", "page through the list"},
			{"space", "select (batch/group)"},
			{"g", "group by project+agent"},
			{"f", "cycle the agent filter"},
			{"F", "search the transcripts"},
			{"/", "filter"},
			{"r", "reload"},
		}},
		{Title: "Transcript (v)", Keys: [][2]string{
			{"n · N", "your next · previous prompt"},
			{"g · G", "top · end"},
			{"m", "view: log / conversation / actions"},
			{"] · [", "pick the next · previous turn with steps"},
			{"enter", "unfold · fold the picked turn"},
			{"e", "steps (⋯): every step / only the answer"},
			{"t", "commands (❯): one per line / summarized"},
			{"r", "reasoning (💭): full / first line only"},
			{"x", "export to Markdown"},
			{"esc", "back to the list"},
		}},
	}
}
