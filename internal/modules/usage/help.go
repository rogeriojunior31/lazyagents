package usage

import (
	"fmt"

	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Help lista as teclas da aba Uso.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Filters", Keys: [][2]string{
			{"p / P", "period: today · 7 · 30 · 90 days · all"},
			{"a / A", "agent: all or just one"},
			{"←/→ · v", "view: day · agent · project · model"},
			{"/", "filter table rows by name"},
			{"esc", "clear the text; again, back to all agents"},
		}},
		{Title: "Usage", Keys: [][2]string{
			{"r", "refresh limits (queries the API)"},
			{"↑/↓ · j/k", "scroll"},
			{"pgup/pgdn · space", "scroll one page"},
			{"g / home", "back to the top"},
		}},
	}
}

// Commands expõe a atualização e os filtros na paleta (module.Commander).
func (m Tab) Commands() []module.Command {
	cmds := []module.Command{{Name: "refresh", Desc: "refresh usage limits", Msg: refreshMsg{}}}
	for i, p := range periods {
		cmds = append(cmds, module.Command{Name: "period " + p.id, Desc: fmt.Sprintf("usage: period %s", p.label), Msg: filterMsg{period: i, setPeriod: true}})
	}
	for i, v := range tabViews {
		cmds = append(cmds, module.Command{Name: "view " + v.id, Desc: fmt.Sprintf("usage by %s", v.label), Msg: filterMsg{view: i, setView: true}})
	}
	return append(cmds, module.Command{Name: "clear", Desc: "usage: clear agent and text", Msg: filterMsg{clear: true}})
}

type refreshMsg struct{}

// filterMsg muda os filtros a partir da paleta.
type filterMsg struct {
	period, view       int
	setPeriod, setView bool
	clear              bool
}

func (f filterMsg) apply(x *filters) {
	if f.setPeriod {
		x.period = f.period
	}
	if f.setView {
		x.view = f.view
	}
	if f.clear {
		x.agent, x.text = "", ""
	}
}
