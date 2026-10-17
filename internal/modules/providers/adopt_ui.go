package providers

import tea "charm.land/bubbletea/v2"

type adoptedMsg struct {
	request  uint64
	adoption Adoption
	err      error
}

func (m *Tab) adoptCmd() tea.Cmd {
	if m.col < 0 || m.col >= len(m.statuses) {
		return nil
	}
	m.validationRequest++
	request, svc, id := m.validationRequest, m.svc, m.statuses[m.col].AgentID
	m.preparing = true
	return func() tea.Msg {
		a, err := svc.AdoptDraft(id)
		return adoptedMsg{request: request, adoption: a, err: err}
	}
}
