// Package components tem widgets reutilizáveis da TUI.
package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Confirm é um dialog sim/não. Default "Não", por segurança.
type Confirm struct {
	Question string
	yes      bool
}

func NewConfirm(question string) Confirm { return Confirm{Question: question} }

// Result é a decisão do usuário.
type Result int

const (
	Pending Result = iota
	Yes
	No
)

func (c Confirm) Update(msg tea.Msg) (Confirm, Result) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, Pending
	}
	switch kp.String() {
	case "y", "Y":
		return c, Yes
	case "n", "N", "esc":
		return c, No
	case "left", "right", "tab", "h", "l":
		c.yes = !c.yes
		return c, Pending
	case "enter":
		if c.yes {
			return c, Yes
		}
		return c, No
	}
	return c, Pending
}

var (
	confirmSel = lipgloss.NewStyle().
			Foreground(theme.Bg).
			Background(theme.Primary).
			Bold(true).Padding(0, 1)
	confirmOff  = lipgloss.NewStyle().Foreground(theme.Subtle).Padding(0, 1)
	confirmHint = lipgloss.NewStyle().Foreground(theme.Subtle)
)

// View desenha o dialog do tamanho do conteúdo (sem quebra de linha).
func (c Confirm) View() string { return c.ViewIn(0, 0) }

// ViewIn desenha o dialog centralizado na área width×height, com a pergunta
// quebrada para caber — caminho comprido é justamente o que o usuário
// precisa ler inteiro antes de dizer sim. 0 = sem área (tamanho natural).
func (c Confirm) ViewIn(width, height int) string {
	yesOpt, noOpt := confirmOff.Render("Sim"), confirmSel.Render("Não")
	if c.yes {
		yesOpt, noOpt = confirmSel.Render("Sim"), confirmOff.Render("Não")
	}
	hint := Keycap("←/→") + confirmHint.Render(" alterna  ") +
		Keycap("enter") + confirmHint.Render(" confirma  ") +
		Keycap("esc") + confirmHint.Render(" cancela")
	question := c.Question
	if width > 0 {
		question = lipgloss.NewStyle().Width(min(72, width) - 4).Render(question)
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		question,
		"",
		yesOpt+"   "+noOpt,
		"",
		hint,
	)
	panel := Panel{Title: "Confirmar", Focused: true, Width: lipgloss.Width(content) + 4}.Render(content)
	if width <= 0 || height <= 0 {
		return panel
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}
