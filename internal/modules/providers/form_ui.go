package providers

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// Form fields, in tab order.
const (
	fName = iota
	fBaseURL
	fModel
	fToken
	fEnvKey
	fWireAPI
	fieldCount
)

var fieldLabels = [fieldCount]string{"name", "endpoint", "model", "token", "env var", "wire api"}

// profileForm creates or edits a profile. The typed token lives only in the
// input until Save; when editing it starts empty, and empty keeps the saved one.
type profileForm struct {
	orig   string // profile being edited ("" = new)
	inputs [fieldCount]textinput.Model
	focus  int
	err    string
}

func newProfileForm(p agent.ProviderProfile, editing bool) *profileForm {
	f := &profileForm{}
	if editing {
		f.orig = p.Name
	}
	values := [fieldCount]string{p.Name, p.BaseURL, p.Model, "", p.EnvKey, p.WireAPI}
	placeholders := [fieldCount]string{
		"work",
		"https://api.example.com/v1",
		"empty = agent default",
		"paste the token (stays masked)",
		"Codex, Pi, Crush: name of the variable holding the token",
		"chat (default), anthropic, responses (Codex, Pi)",
	}
	if editing && p.HasToken {
		placeholders[fToken] = "empty = keep the saved token"
	}
	for i := range f.inputs {
		in := components.NewInput()
		in.Prompt = ""
		in.Placeholder = placeholders[i]
		in.SetValue(values[i])
		in.SetWidth(44)
		if i == fToken {
			in.EchoMode = textinput.EchoPassword
			in.EchoCharacter = '•'
		}
		f.inputs[i] = in
	}
	f.inputs[0].Focus()
	return f
}

func (f *profileForm) profile() agent.ProviderProfile {
	v := func(i int) string { return strings.TrimSpace(f.inputs[i].Value()) }
	return agent.ProviderProfile{
		Name: v(fName), BaseURL: v(fBaseURL), Model: v(fModel),
		Token: v(fToken), EnvKey: v(fEnvKey), WireAPI: v(fWireAPI),
	}
}

func (f *profileForm) setFocus(i int) {
	f.inputs[f.focus].Blur()
	f.focus = (i + fieldCount) % fieldCount
	f.inputs[f.focus].Focus()
}

// formResult tells the tab what to do after a key in the form.
type formResult int

const (
	formEditing formResult = iota
	formSubmit
	formCancel
)

func (f *profileForm) update(msg tea.KeyPressMsg) (formResult, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return formCancel, nil
	case "tab", "down":
		f.setFocus(f.focus + 1)
		return formEditing, nil
	case "shift+tab", "up":
		f.setFocus(f.focus - 1)
		return formEditing, nil
	case "enter", "ctrl+s":
		return formSubmit, nil
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return formEditing, cmd
}

func (f *profileForm) paste(msg tea.PasteMsg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return cmd
}

func (f *profileForm) view(width, height int) string {
	title := "New provider profile"
	if f.orig != "" {
		title = fmt.Sprintf("Edit profile %s", f.orig)
	}
	w := min(width, 72)
	panel := components.Panel{Title: title, Focused: true, Width: w}
	inner := panel.ContentWidth()
	errText := ""
	if f.err != "" {
		errText = lipgloss.NewStyle().Width(inner).Render(kit.StErr.Render(f.err))
	}
	errorH := lipgloss.Height(errText)
	if errText == "" {
		errorH = 0
	}
	start, end := kit.Window(f.focus, fieldCount, max(1, height-4-errorH))
	var rows []string
	for i := start; i < end; i++ {
		f.inputs[i].SetWidth(max(1, inner-12))
		// SetWidth does not recompute the text window; keep the cursor visible.
		f.inputs[i].SetCursor(f.inputs[i].Position())
		label := kit.CardLabel.Render(padRight(fieldLabels[i], 11))
		if i == f.focus {
			label = kit.StShared.Render("▸ ") + kit.StTitle.Render(padRight(fieldLabels[i], 9))
		}
		rows = append(rows, label+f.inputs[i].View())
	}
	if start > 0 || end < fieldCount {
		panel.Title += fmt.Sprintf(" · %d/%d", f.focus+1, fieldCount)
	}
	if errText != "" {
		rows = append(rows, errText)
	}
	rows = append(rows, "", kit.Hints(inner, [2]string{"esc", "back"}, [2]string{"enter", "save"}, [2]string{"tab", "field"}))
	return panel.Render(strings.Join(rows, "\n"))
}

func padRight(s string, n int) string {
	return s + strings.Repeat(" ", max(0, n-len([]rune(s))))
}
