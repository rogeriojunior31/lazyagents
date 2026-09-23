package sessions

import "github.com/rogeriojunior31/lazyagents/internal/tui/module"

// Help lista as teclas da aba Sessões, agrupadas por assunto.
func (m Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Sessões", Keys: [][2]string{
			{"enter", "retoma"},
			{"v", "lê o transcript"},
			{"R", "retoma em outra pasta"},
			{"c", "mostra o comando"},
			{"m", "apelido (vazio remove)"},
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
		{Title: "Transcript (v)", Keys: [][2]string{
			{"n · N", "próximo · anterior prompt seu"},
			{"g · G", "início · fim"},
			{"t", "comandos (❯): um por linha / resumidos"},
			{"r", "raciocínio (💭): inteiro / só a 1ª linha"},
			{"x", "exporta em Markdown"},
			{"esc", "volta à lista"},
		}},
	}
}
