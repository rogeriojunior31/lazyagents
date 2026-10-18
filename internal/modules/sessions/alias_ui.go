package sessions

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

// openAlias opens the alias input prefilled with the current one.
func (m Tab) openAlias(s agent.Session) Tab {
	inp := components.NewInput()
	inp.Placeholder = "session alias"
	inp.SetWidth(60)
	inp.SetValue(s.Alias)
	inp.Focus()
	m.aliasInput, m.aliasTarget, m.mode = inp, s, sessModeAlias
	return m
}

func (m Tab) updateAlias(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.aliasInput.Value() != m.aliasTarget.Alias {
			c := components.NewConfirm("Discard the unsaved alias?\nChoose No to keep editing. The saved alias is unchanged.")
			m.discardAlias = &c
			return m, nil
		}
		m.mode = sessModeList
		return m, nil
	case "enter":
		m.writing = true
		svc, s, alias := m.svc, m.aliasTarget, strings.TrimSpace(m.aliasInput.Value())
		return m, func() tea.Msg {
			return aliasDoneMsg{key: s.AgentID + ":" + s.ID, alias: alias, err: svc.SetAlias(s, alias)}
		}
	}
	var cmd tea.Cmd
	m.aliasInput, cmd = m.aliasInput.Update(msg)
	return m, cmd
}
