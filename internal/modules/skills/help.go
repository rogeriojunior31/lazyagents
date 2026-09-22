package skills

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Skills, agrupadas por assunto.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Skills", Keys: [][2]string{
			{"enter", "lê o SKILL.md"},
			{"e", "edita no $EDITOR"},
			{"n", "nova skill"},
			{"o", "adota p/ biblioteca"},
			{"A", "adota todas as locais"},
			{"d", "remove (com backup)"},
			{"i", "instala (GitHub/pasta/zip)"},
			{"S", "busca no GitHub (registry)"},
		}},
		{Title: "Ativação", Keys: [][2]string{
			{"1-9", "alterna no agente N"},
			{"space", "alterna em todos"},
			{"a", "ativa em todos"},
			{"x", "desativa em todos"},
		}},
		{Title: "Perfis & updates", Keys: [][2]string{
			{"p", "perfis"},
			{"u", "atualiza esta skill"},
			{"U", "verifica updates"},
			{"b", "backups"},
		}},
		{Title: "Lista", Keys: [][2]string{
			{"←/→", "foca lista/detalhe"},
			{"/", "filtra"},
			{"r", "recarrega"},
		}},
	}
}
