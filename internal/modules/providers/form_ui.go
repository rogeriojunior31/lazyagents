package providers

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
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
	template   *connectionTemplate
	helpScroll int
	hasToken   bool
	orig       string // profile being edited ("" = new)
	inputs     [fieldCount]textinput.Model
	focus      int
	err        string
}

func newProfileForm(p agent.ProviderProfile, editing bool) *profileForm {
	f := &profileForm{hasToken: p.Redacted().HasToken}
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
		Token: v(fToken), HasToken: f.hasToken && v(fToken) == "", EnvKey: v(fEnvKey), WireAPI: v(fWireAPI),
	}
}

func (f *profileForm) setFocus(i int) {
	f.helpScroll = 0
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
	formValidate
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
	case "ctrl+v":
		return formValidate, nil
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
	if f.template != nil {
		title = f.template.name
	}
	if width >= kit.SideDetailWidth {
		sp := kit.SplitDetail(min(width, 160), height)
		left := f.fieldsView(title, sp.ListW, height, false)
		vp := f.helpViewport(sp.DetailW-4, max(1, height-2))
		help := components.Panel{Title: f.helpTitle(), Width: sp.DetailW, Height: height}.Render(vp.View())
		return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", help)
	}
	return f.fieldsView(title, min(width, 80), height, true)
}

func (f *profileForm) fieldsView(title string, width, height int, stacked bool) string {
	panel := components.Panel{Title: title, Focused: true, Width: width}
	inner := panel.ContentWidth()
	helpH := 0
	fieldRows := height - 4
	if stacked {
		helpH = min(8, max(1, height/3))
		fieldRows = height - 5 - helpH
	}
	start, end := kit.Window(f.focus, fieldCount, max(1, fieldRows))
	var rows []string
	for i := start; i < end; i++ {
		f.inputs[i].SetWidth(max(1, inner-12))
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
	if stacked {
		// At very short heights, leave one input and all navigation visible.
		helpH = max(1, min(helpH, height-5-(end-start)))
		rows = append(rows, kit.StTitle.Render(f.helpTitle()), f.helpViewport(inner, helpH).View())
	}
	rows = append(rows, "", kit.Hints(inner, [2]string{"esc", "back"}, [2]string{"enter", "save"}, [2]string{"tab", "field"}, [2]string{"ctrl+v", "check"}))
	return panel.Render(strings.Join(rows, "\n"))
}

func (f *profileForm) helpTitle() string {
	return fmt.Sprintf("About %s · shift+↑↓ scroll", fieldLabels[f.focus])
}

var fieldHelp = [fieldCount]string{
	"Name this connection in your library (up to 40 characters). Saving a profile does not apply it to any agent.",
	"Use the API base URL, including a required path such as /v1. Do not append an operation such as /responses or /chat/completions. Claude gateways use the base URL supplied by the gateway operator.",
	"Enter the exact model ID offered by the endpoint. Pi and Crush require a model; Codex and Claude can use their native default in a custom profile. Use a model that supports the agent's tools. No model discovery or inference is performed here.",
	"A saved token is masked and kept in the private profile library. Claude writes it to native settings. Pi and Crush prefer it over env var. Codex never receives it: use env var instead. When editing, an empty token keeps the saved token.",
	"Enter only a variable name, such as OPENAI_API_KEY, never its value or a $ prefix. Export it in the agent's environment before starting a new session. A name does not prove authentication. Claude cannot resolve this field into its saved token.",
	"Match the endpoint protocol: responses for Codex/Pi, chat for Pi/Crush, anthropic for Pi/Crush. Claude always speaks Anthropic Messages and ignores this selector. An empty value means Responses in Codex and Chat Completions in Pi/Crush. No protocol conversion is provided.",
}

func (f *profileForm) helpText() string {
	text := fieldHelp[f.focus]
	if f.template != nil {
		t := f.template
		detail := t.required
		switch f.focus {
		case fBaseURL, fWireAPI:
			detail = t.protocol
		case fToken, fEnvKey:
			detail = t.credential
		}
		if t.profile.Name != "" {
			text = fmt.Sprintf("Template targets: %s\n%s\n\n%s", strings.Join(t.agents, ", "), detail, text)
		} else {
			text += "\n\n" + detail
		}
		text += fmt.Sprintf("\n\nTemplate guidance reviewed %s:\n%s\nAll fields are editable; changes need a fresh compatibility check.", t.reviewed, t.source)
	}
	if f.err != "" {
		text = kit.StErr.Render(f.err) + "\n\n" + text
	}
	return text
}

func (f *profileForm) helpViewport(width, height int) viewport.Model {
	vp := viewport.New(viewport.WithWidth(max(1, width)), viewport.WithHeight(max(1, height)))
	vp.SetContent(kit.Wrap(f.helpText(), max(1, width), ""))
	vp.SetYOffset(f.helpScroll)
	return vp
}

func (f *profileForm) scrollHelp(msg tea.Msg, width, height int) {
	w, h := max(1, min(width, 80)-4), min(8, max(1, height/3))
	if width >= kit.SideDetailWidth {
		sp := kit.SplitDetail(min(width, 160), height)
		w, h = sp.DetailW-4, max(1, height-2)
	} else {
		h = max(1, min(h, height-6))
	}
	vp := f.helpViewport(w, h)
	if key, ok := msg.(tea.KeyPressMsg); ok {
		msg = kit.DetailScrollMsg(key)
	}
	vp, _ = vp.Update(msg)
	f.helpScroll = vp.YOffset()
}

func (f *profileForm) validateTemplate() bool {
	if f.template == nil || f.template.profile.Name == "" {
		return true
	}
	p := f.profile()
	switch {
	case p.BaseURL == "":
		f.setFocus(fBaseURL)
		f.err = "This connection template needs an endpoint"
	case p.Model == "":
		f.setFocus(fModel)
		f.err = "This connection template needs an explicit model ID"
	case f.template.profile.Name == "anthropic-gateway" && p.Token == "" && !p.HasToken:
		f.setFocus(fToken)
		f.err = "This Claude gateway template needs a saved bearer token; env var cannot supply it"
	default:
		return true
	}
	return false
}

func padRight(s string, n int) string {
	return s + strings.Repeat(" ", max(0, n-len([]rune(s))))
}
