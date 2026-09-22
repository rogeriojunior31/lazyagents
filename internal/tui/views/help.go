package views

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// HelpGroup é o bloco de teclas do modal de ajuda (M7.1). Cada view expõe seus
// grupos via Help(), sempre refletindo as teclas realmente tratadas.
type HelpGroup = module.HelpGroup

// Help lista as teclas da aba Skills, agrupadas por assunto.
func (m Skills) Help() []HelpGroup {
	return []HelpGroup{
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

// Help lista as teclas da aba Sessões, agrupadas por assunto.
func (m Sessions) Help() []HelpGroup {
	return []HelpGroup{
		{Title: "Sessões", Keys: [][2]string{
			{"enter", "retoma"},
			{"v", "transcript"},
			{"x", "exporta transcript (no modo leitura)"},
			{"R", "retoma em outra pasta"},
			{"c", "mostra o comando"},
			{"d", "deleta (com backup)"},
		}},
		{Title: "Lista", Keys: [][2]string{
			{"←/→", "foca lista/detalhe"},
			{"space", "seleciona (lote/grupo)"},
			{"g", "agrupa por agente+projeto"},
			{"f", "cicla filtro por agente"},
			{"F", "busca nos transcripts"},
			{"/", "filtra"},
			{"r", "recarrega"},
		}},
	}
}

// Help para a aba Agentes — somente leitura, sem teclas próprias.
func (m Agents) Help() []HelpGroup {
	return []HelpGroup{
		{Title: "Agentes", Keys: [][2]string{
			{"—", "somente leitura"},
		}},
	}
}
