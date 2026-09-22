package kit

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

var (
	selTitle = lipgloss.NewStyle().Background(theme.Sel).Foreground(theme.Text).Bold(true)
	selDesc  = lipgloss.NewStyle().Background(theme.Sel).Foreground(theme.Subtle)
)

// PlainDelegate renderiza itens em 2 linhas (título + descrição) SEM o realce
// de runas do delegate padrão durante o filtro — esse realce fatia o título
// por índice de runa e corrompe as sequências ANSI do badge da matriz e da
// tag colorida de agente.
type PlainDelegate struct{}

func (PlainDelegate) Height() int                         { return 2 }
func (PlainDelegate) Spacing() int                        { return 1 }
func (PlainDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

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

func (PlainDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(interface {
		Title() string
		Description() string
	})
	if !ok {
		return
	}
	width := m.Width()
	if width < 6 {
		width = 6
	}
	if index == m.Index() {
		// Linha inteira realçada. Removemos o ANSI do título (badges/tag de
		// agente) porque os resets internos furam o background — a cor volta
		// nas linhas não selecionadas e no painel de detalhe.
		t := ansi.Truncate(ansi.Strip(it.Title()), width-2, "…")
		d := ansi.Truncate(ansi.Strip(it.Description()), width-2, "…")
		fmt.Fprintf(w, "%s\n%s", selTitle.Width(width).Render("› "+t), selDesc.Width(width).Render("  "+d))
	} else {
		title := ansi.Truncate(it.Title(), width-2, "…")
		desc := StHint.Render(ansi.Truncate(it.Description(), width-2, "…"))
		fmt.Fprintf(w, "  %s\n  %s", title, desc)
	}
}
