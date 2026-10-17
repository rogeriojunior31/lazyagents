package providers

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type templatePicker struct{ cursor, scroll int }

func (m *Tab) templateKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.templates
	switch msg.String() {
	case "esc":
		m.templates = nil
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
		p.scroll = 0
	case "down", "j":
		p.cursor = min(len(connectionTemplates)-1, p.cursor+1)
		p.scroll = 0
	case "enter":
		t := &connectionTemplates[p.cursor]
		if t.defaults {
			return m.chooseDefaults()
		}
		m.form = newProfileForm(t.profile, false)
		m.form.template = t
		if t.profile.Name != "" {
			m.form.inputs[fModel].Placeholder = "exact model ID (required)"
			m.form.setFocus(fModel)
			if t.profile.BaseURL == "" {
				m.form.setFocus(fBaseURL)
			}
		}
		m.templates = nil
	default:
		vp, _ := m.templateViewport().Update(kit.DetailScrollMsg(msg))
		p.scroll = vp.YOffset()
	}
	return nil
}

func (m Tab) templateLayout() (int, int, string) {
	w := max(1, min(m.width, 92)-4)
	footer := kit.Wrap("↑↓ template · shift+↑↓ details", w, "") + "\n" + kit.Hints(w, [2]string{"enter", "choose"}, [2]string{"esc", "back"})
	rows := min(len(connectionTemplates), max(1, (m.height-4-lipgloss.Height(footer))/2))
	return w, rows, footer
}

func (m Tab) templateViewport() viewport.Model {
	p := m.templates
	t := connectionTemplates[p.cursor]
	w, rows, footer := m.templateLayout()
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(max(1, m.height-4-rows-lipgloss.Height(footer))))
	names := make([]string, 0, len(t.agents))
	for _, id := range t.agents {
		names = append(names, m.targetName(id))
	}
	text := fmt.Sprintf("%s\n\nAgents: %s\nProtocol: %s\nRequired: %s\nCredentials: %s\n\nSource reviewed %s (configuration guidance; no live connection test):\n%s", t.summary, strings.Join(names, ", "), t.protocol, t.required, t.credential, t.reviewed, t.source)
	vp.SetContent(kit.Wrap(text, w, ""))
	vp.SetYOffset(p.scroll)
	return vp
}

func (m Tab) templatesView() string {
	w, rows, footer := m.templateLayout()
	panel := components.Panel{Title: "New provider · choose a starting point", Focused: true, Width: min(m.width, 92)}
	lines := []string{kit.StHint.Render("Choose first, then edit · no files changed")}
	start, end := kit.Window(m.templates.cursor, len(connectionTemplates), rows)
	for i := start; i < end; i++ {
		mark := "  "
		if i == m.templates.cursor {
			mark = "▸ "
		}
		lines = append(lines, ansi.Truncate(mark+connectionTemplates[i].name, w, "…"))
	}
	lines = append(lines, "", m.templateViewport().View(), footer)
	return panel.Render(strings.Join(lines, "\n"))
}

func (m *Tab) chooseDefaults() tea.Cmd {
	m.templates = nil
	return m.prepareClearCmd(nil, true)
}
