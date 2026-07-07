package views

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"lazyskills/internal/tui/theme"
)

var delBar = lipgloss.NewStyle().Foreground(theme.Primary)

// plainDelegate renderiza itens em 2 linhas (título + descrição) SEM o realce
// de runas do delegate padrão durante o filtro — esse realce fatia o título
// por índice de runa e corrompe as sequências ANSI do badge da matriz e da
// tag colorida de agente.
type plainDelegate struct{}

func (plainDelegate) Height() int                         { return 2 }
func (plainDelegate) Spacing() int                        { return 1 }
func (plainDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

// feedTextToList injeta texto colado no filtro da lista como se fosse
// digitado — a list não trata tea.PasteMsg, mas refiltra a cada tecla.
func feedTextToList(l list.Model, s string) (list.Model, tea.Cmd) {
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

func (plainDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(interface {
		Title() string
		Description() string
	})
	if !ok {
		return
	}
	width := m.Width() - 2
	if width < 4 {
		width = 4
	}
	title := ansi.Truncate(it.Title(), width, "…")
	desc := stHint.Render(ansi.Truncate(it.Description(), width, "…"))
	if index == m.Index() {
		fmt.Fprintf(w, "%s%s\n%s%s", delBar.Render("│ "), title, delBar.Render("│ "), desc)
	} else {
		fmt.Fprintf(w, "  %s\n  %s", title, desc)
	}
}
