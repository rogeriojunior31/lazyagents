package views

// HelpGroup é um bloco de teclas para o modal de ajuda (M7.1): um título e as
// linhas {tecla, descrição}. Cada view expõe seus grupos via Help(), sempre
// refletindo as teclas realmente tratadas no Update — nada inventado aqui.
type HelpGroup struct {
	Title string
	Keys  [][2]string
}

// Help lista as teclas da aba Skills, agrupadas por assunto.
func (m Skills) Help() []HelpGroup {
	return []HelpGroup{
		{"Skills", [][2]string{
			{"enter", "lê o SKILL.md"},
			{"e", "edita no $EDITOR"},
			{"n", "nova skill"},
			{"o", "adota p/ biblioteca"},
			{"A", "adota todas as locais"},
			{"d", "remove (com backup)"},
			{"i", "instala (GitHub/pasta/zip)"},
		}},
		{"Ativação", [][2]string{
			{"1-9", "alterna no agente N"},
			{"space", "alterna em todos"},
			{"a", "ativa em todos"},
			{"x", "desativa em todos"},
		}},
		{"Perfis & updates", [][2]string{
			{"p", "perfis"},
			{"u", "atualiza esta skill"},
			{"U", "verifica updates"},
			{"b", "backups"},
		}},
		{"Lista", [][2]string{
			{"←/→", "foca lista/detalhe"},
			{"/", "filtra"},
			{"r", "recarrega"},
		}},
	}
}

// Help lista as teclas da aba Sessões, agrupadas por assunto.
func (m Sessions) Help() []HelpGroup {
	return []HelpGroup{
		{"Sessões", [][2]string{
			{"enter", "retoma"},
			{"v", "transcript"},
			{"R", "retoma em outra pasta"},
			{"c", "mostra o comando"},
			{"d", "deleta (com backup)"},
		}},
		{"Lista", [][2]string{
			{"←/→", "foca lista/detalhe"},
			{"space", "seleciona (lote)"},
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
		{"Agentes", [][2]string{
			{"—", "somente leitura"},
		}},
	}
}
