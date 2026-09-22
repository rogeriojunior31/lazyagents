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

func (c Confirm) View() string {
	yesOpt, noOpt := confirmOff.Render("Sim"), confirmSel.Render("Não")
	if c.yes {
		yesOpt, noOpt = confirmSel.Render("Sim"), confirmOff.Render("Não")
	}
	hint := Keycap("←/→") + confirmHint.Render(" alterna  ") +
		Keycap("enter") + confirmHint.Render(" confirma  ") +
		Keycap("esc") + confirmHint.Render(" cancela")
	content := lipgloss.JoinVertical(lipgloss.Left,
		c.Question,
		"",
		yesOpt+"   "+noOpt,
		"",
		hint,
	)
	return Panel{Title: "Confirmar", Width: lipgloss.Width(content) + 4}.Render(content)
}
