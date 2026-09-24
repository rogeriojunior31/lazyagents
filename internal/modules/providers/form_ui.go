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

// Campos do formulário, na ordem do tab.
const (
	fName = iota
	fBaseURL
	fModel
	fToken
	fEnvKey
	fWireAPI
	fieldCount
)

var fieldLabels = [fieldCount]string{"nome", "endpoint", "modelo", "token", "variável", "wire api"}

// profileForm cria ou edita um perfil. O token digitado só existe no input
// até o Save; na edição ele começa vazio, e vazio mantém o salvo (Edit).
type profileForm struct {
	orig   string // nome do perfil editado ("" = novo)
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
		"trabalho",
		"https://api.exemplo.com/v1",
		"vazio = padrão do agente",
		"cole o token (fica mascarado)",
		"Codex: nome da variável com o token",
		"Codex: responses",
	}
	if editing && p.HasToken {
		placeholders[fToken] = "vazio = mantém o token salvo"
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

// formResult diz o que a aba faz depois de uma tecla no formulário.
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
	title := "Novo perfil de provedor"
	if f.orig != "" {
		title = "Editar perfil " + f.orig
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
		// SetWidth não recalcula a janela do texto; preserve o cursor visível.
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
	rows = append(rows, "", kit.Hints(inner, [2]string{"esc", "volta"}, [2]string{"enter", "salva"}, [2]string{"tab", "campo"}))
	return panel.Render(strings.Join(rows, "\n"))
}

func padRight(s string, n int) string {
	return s + strings.Repeat(" ", max(0, n-len([]rune(s))))
}
