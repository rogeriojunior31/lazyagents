package skills

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

type confirmKind int

const (
	confirmKindRemove confirmKind = iota
	confirmKindUpdate
	confirmKindApplyProfile
	confirmKindUpdateAll
	confirmKindRestore
	confirmKindAdoptAll
	confirmKindDeleteProfile
)

func (m Tab) updateConfirm(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	var res components.Result
	m.confirm, res = m.confirm.Update(msg, m.width, m.height)
	switch res {
	case components.Yes:
		m.mode = skModeList
		switch m.ckind {
		case confirmKindUpdate:
			sel := m.pendingUpdate
			svc := m.svc
			spin := m.beginSpin(fmt.Sprintf("updating %s…", sel.Name))
			return m, tea.Batch(spin, func() tea.Msg {
				return updateDoneMsg{name: sel.Name, err: svc.Update(sel)}
			})
		case confirmKindApplyProfile:
			name := m.pendingProfile
			svc, agents := m.svc, m.agents
			return m, func() tea.Msg {
				return profileApplyDoneMsg{name: name, err: svc.ApplyProfile(name, agents)}
			}
		case confirmKindDeleteProfile:
			name, svc := m.pendingProfile, m.svc
			m.mode = skModeProfiles
			return m, func() tea.Msg {
				return profileDeleteMsg{name: name, err: svc.DeleteProfile(name)}
			}
		case confirmKindUpdateAll:
			return m, tea.Batch(m.beginSpin("updating skills…"), m.updateAllCmd())
		case confirmKindRestore:
			return m, m.restoreBackupCmd()
		case confirmKindAdoptAll:
			svc, skills, agents := m.svc, m.skills, m.agents
			spin := m.beginSpin("adopting local skills…")
			return m, tea.Batch(spin, func() tea.Msg {
				adopted, errs := svc.AdoptAll(skills, agents)
				return adoptAllDoneMsg{adopted: adopted, errs: errs}
			})
		case confirmKindRemove:
			sel := m.pendingRemove
			return m, m.opCmd(fmt.Sprintf("removed (backup in %s)", core.Tilde(m.svc.Paths().BackupsDir(), m.svc.Paths().Home)), func() error {
				return m.svc.Remove(sel, m.agents)
			})
		}
	case components.No:
		m.mode = skModeList
		if m.ckind == confirmKindDeleteProfile {
			m.mode = skModeProfiles
		}
	}
	return m, nil
}
