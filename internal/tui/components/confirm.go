// Package components tem widgets reutilizáveis da TUI.
package components

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Confirm é um dialog sim/não. Default "Não", por segurança.
type Confirm struct {
	Question string
	yes      bool
	scroll   int
}

func NewConfirm(question string) Confirm { return Confirm{Question: question} }

// Result é a decisão do usuário.
type Result int

const (
	Pending Result = iota
	Yes
	No
)

func (c Confirm) Update(msg tea.Msg, width, height int) (Confirm, Result) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if _, wheel := msg.(tea.MouseWheelMsg); wheel {
			c = c.scrollQuestion(msg, width, height)
		}
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
	return c.scrollQuestion(msg, width, height), Pending
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
	vp := c.questionViewport(width, height)
	hint := "←→ alterna · enter · esc cancela"
	if width <= 0 || width >= 64 {
		hint = Keycap("←/→") + confirmHint.Render(" alterna  ") + Keycap("enter") + confirmHint.Render(" confirma  ") + Keycap("esc") + confirmHint.Render(" cancela")
	}
	title := "Confirmar"
	if vp.TotalLineCount() > vp.VisibleLineCount() {
		title += fmt.Sprintf(" · ↑↓ rola · %.0f%%", vp.ScrollPercent()*100)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, vp.View(), "", yesOpt+"   "+noOpt, hint)
	w := lipgloss.Width(content) + 4
	if width > 0 {
		w = min(width, 72)
	}
	panel := Panel{Title: title, Focused: true, Width: w}.Render(content)
	if width <= 0 || height <= 0 {
		return panel
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func (c Confirm) questionViewport(width, height int) viewport.Model {
	w := max(1, lipgloss.Width(c.Question))
	if width > 0 {
		w = max(1, min(width, 72)-4)
	}
	question := lipgloss.NewStyle().Width(w).Render(c.Question)
	h := lipgloss.Height(question)
	if height > 0 {
		h = min(h, max(1, height-5))
	}
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(question)
	vp.SetYOffset(c.scroll)
	return vp
}

func (c Confirm) scrollQuestion(msg tea.Msg, width, height int) Confirm {
	vp := c.questionViewport(width, height)
	vp, _ = vp.Update(msg)
	c.scroll = vp.YOffset()
	return c
}
