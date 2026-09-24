package providers

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Provedores.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Provedores", Keys: [][2]string{
			{"←/→", "foca lista/detalhe"},
			{"↑↓ · pgup/pgdn", "rola o detalhe focado"},
			{"↑/↓ · j/k", "escolher perfil"},
			{"1-9", "aplica no agente N (de novo remove)"},
			{"space · a", "aplica em todos os instalados"},
			{"x", "remove o provedor de todos"},
			{"n", "cria um perfil (formulário)"},
			{"e", "edita o perfil; token vazio mantém o salvo"},
			{"d", "apaga o perfil da biblioteca"},
			{"r", "recarrega"},
		}},
		{Title: "Criar perfil", Keys: [][2]string{
			{"lazyagents provider add", "cria um perfil pela CLI"},
			{"--token -", "lê o token da entrada padrão"},
		}},
	}
}

// Commands expõe a limpeza na paleta (module.Commander).
func (m Tab) Commands() []module.Command {
	return []module.Command{
		{Name: "new", Desc: "cria um perfil de provedor", Msg: newProfileMsg{}},
		{Name: "clear", Desc: "remove o provedor de todos os agentes", Msg: clearAllMsg{}},
	}
}
