package kit

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

func StyleList(l *list.Model) {
	l.Styles.Filter = components.InputStyles()
	l.FilterInput.SetStyles(components.InputStyles())
	l.Styles.NoItems = StHint
	l.Styles.Spinner = StTitle
	l.Styles.StatusEmpty = StHint
	l.Styles.StatusBar = StHint
}

// ListView troca o estado vazio fixo em inglês do bubbles/list ("No items.")
// por uma mensagem em PT-BR. Durante a digitação do filtro a lista segue
// renderizando normalmente (o input do filtro fica visível).
func ListView(l list.Model, empty string) string {
	if len(l.VisibleItems()) == 0 && l.FilterState() != list.Filtering {
		if l.FilterState() == list.FilterApplied {
			empty = fmt.Sprintf("Nothing found for “%s”.", l.FilterValue())
		}
		return StHint.Render(empty)
	}
	return l.View()
}

// FeedTextToList injeta texto colado no filtro da lista como se fosse
// digitado — a list não trata tea.PasteMsg, mas refiltra a cada tecla.
func FeedTextToList(l list.Model, s string) (list.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		var cmd tea.Cmd
		l, cmd = l.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		cmds = append(cmds, cmd)
	}
	return l, tea.Batch(cmds...)
}
