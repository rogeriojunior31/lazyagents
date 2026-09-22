package sessions

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Sessões, agrupadas por assunto.
func (m Sessions) Help() []module.HelpGroup {
	return []module.HelpGroup{
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
