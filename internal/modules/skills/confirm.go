package skills

import (
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
)

func (m Tab) updateConfirm(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	var res components.Result
	m.confirm, res = m.confirm.Update(msg)
	switch res {
	case components.Yes:
		m.mode = skModeList
		switch m.ckind {
		case confirmKindUpdate:
			sel := m.pendingUpdate
			svc := m.svc
			spin := m.beginSpin("atualizando " + sel.Name + "…")
			return m, tea.Batch(spin, func() tea.Msg {
				return updateDoneMsg{name: sel.Name, err: svc.Update(sel)}
			})
		case confirmKindApplyProfile:
			name := m.pendingProfile
			svc, agents := m.svc, m.agents
			return m, func() tea.Msg {
				return profileApplyDoneMsg{name: name, err: svc.ApplyProfile(name, agents)}
			}
		case confirmKindUpdateAll:
			return m, tea.Batch(m.beginSpin("atualizando skills…"), m.updateAllCmd())
		case confirmKindRestore:
			return m, m.restoreBackupCmd()
		case confirmKindAdoptAll:
			svc, skills, agents := m.svc, m.skills, m.agents
			spin := m.beginSpin("adotando skills locais…")
			return m, tea.Batch(spin, func() tea.Msg {
				adopted, errs := svc.AdoptAll(skills, agents)
				return adoptAllDoneMsg{adopted: adopted, errs: errs}
			})
		default:
			sel := m.pendingRemove
			return m, m.opCmd("removida (backup em "+core.Tilde(m.svc.Paths().BackupsDir(), m.svc.Paths().Home)+")", func() error {
				return m.svc.Remove(sel, m.agents)
			})
		}
	case components.No:
		m.mode = skModeList
	}
	return m, nil
}
