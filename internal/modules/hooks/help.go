package hooks

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Hooks.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Hooks", Keys: [][2]string{
			{"↑/↓ · j/k", "escolher hook"},
			{"1-9", "instala no agente N (de novo remove)"},
			{"space · a", "instala em todos que disparam o evento"},
			{"enter", "escolhe os comandos do pacote (space liga/desliga, esc volta)"},
			{"x", "remove de todos os agentes"},
			{"d", "apaga o hook da biblioteca"},
			{"pgup/pgdn · ctrl+u/d", "rola o detalhe"},
			{"r", "recarrega"},
		}},
		{Title: "Criar hook", Keys: [][2]string{
			{"lazyagents hooks add", "cria um hook pela CLI"},
			{"~/.local/share/lazyagents/hooks", "um JSON por hook, editável à mão"},
		}},
	}
}
