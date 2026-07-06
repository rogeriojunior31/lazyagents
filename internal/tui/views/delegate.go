package views

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var delBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))

// plainDelegate renderiza itens em 2 linhas (título + descrição) SEM o realce
// de runas do delegate padrão durante o filtro — esse realce fatia o título
// por índice de runa e corrompe as sequências ANSI do badge da matriz e da
// tag colorida de agente.
type plainDelegate struct{}

func (plainDelegate) Height() int                         { return 2 }
func (plainDelegate) Spacing() int                        { return 1 }
func (plainDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

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
