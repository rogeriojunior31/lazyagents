package usage

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Uso.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Filtros", Keys: [][2]string{
			{"p / P", "período: hoje · 7 · 30 · 90 dias · tudo"},
			{"a / A", "agente: todos ou um só"},
			{"←/→ · v", "visão: dia · agente · projeto · modelo"},
			{"/", "filtra as linhas da tabela pelo nome"},
			{"esc", "limpa o texto; de novo, volta a todos os agentes"},
		}},
		{Title: "Uso", Keys: [][2]string{
			{"r", "atualiza os limites (consulta a API)"},
			{"↑/↓ · j/k", "rolar"},
			{"pgup/pgdn · space", "rolar uma página"},
			{"g / home", "voltar ao início"},
		}},
	}
}

// Commands expõe a atualização e os filtros na paleta (module.Commander).
func (m Tab) Commands() []module.Command {
	cmds := []module.Command{{Name: "refresh", Desc: "atualiza os limites de uso", Msg: refreshMsg{}}}
	for i, p := range periods {
		cmds = append(cmds, module.Command{Name: "period " + p.id, Desc: "uso: período " + p.label, Msg: filterMsg{period: i, setPeriod: true}})
	}
	for i, v := range tabViews {
		cmds = append(cmds, module.Command{Name: "view " + v.id, Desc: "uso por " + v.label, Msg: filterMsg{view: i, setView: true}})
	}
	return append(cmds, module.Command{Name: "clear", Desc: "uso: limpa agente e texto", Msg: filterMsg{clear: true}})
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
