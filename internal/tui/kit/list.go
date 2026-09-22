package kit

import (
	"charm.land/bubbles/v2/list"
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
			empty = "Nada encontrado para “" + l.FilterValue() + "”."
		}
		return StHint.Render(empty)
	}
	return l.View()
}
