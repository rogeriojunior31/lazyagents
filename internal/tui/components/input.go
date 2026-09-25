package components

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

func InputStyles() textinput.Styles {
	s := textinput.DefaultDarkStyles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(theme.Text)
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.Subtle)
	s.Focused.Suggestion = lipgloss.NewStyle().Foreground(theme.Subtle)
	s.Blurred = s.Focused
	s.Cursor.Color = theme.Primary
	return s
}

func NewInput() textinput.Model {
	in := textinput.New()
	in.SetStyles(InputStyles())
	return in
}

// InputView keeps the cursor inside the usable width, prompt and cursor included.
func InputView(in textinput.Model, width int) string {
	in.SetWidth(max(1, width-lipgloss.Width(in.Prompt)-1))
	in.SetCursor(in.Position())
	return in.View()
}
