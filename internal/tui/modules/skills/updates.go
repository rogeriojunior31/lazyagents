package skills

import (
	tea "charm.land/bubbletea/v2"
)

func (m Skills) checkUpdatesCmd() tea.Cmd {
	svc, skills := m.svc, m.skills
	return func() tea.Msg {
		checks, err := svc.CheckUpdates(skills)
		return checkUpdatesMsg{checks: checks, err: err}
	}
}

func (m Skills) updateAllCmd() tea.Cmd {
	svc, checks := m.svc, m.updateChecks
	return func() tea.Msg {
		updated, skipped, errs := svc.UpdateAll(checks)
		return updateAllMsg{updated: updated, skipped: skipped, errs: errs}
	}
}
