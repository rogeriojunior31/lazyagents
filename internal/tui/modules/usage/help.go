package usage

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Uso.
func (m Usage) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Uso", Keys: [][2]string{
			{"r", "atualiza os limites (consulta a API)"},
			{"↑/↓ · j/k", "rolar"},
			{"g / home", "voltar ao início"},
		}},
	}
}

// Commands expõe a atualização na paleta (module.Commander).
func (m Usage) Commands() []module.Command {
	return []module.Command{{Name: "refresh", Desc: "atualiza os limites de uso", Msg: refreshMsg{}}}
}

type refreshMsg struct{}
