package skills

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

type confirmKind int

const (
	confirmKindRemove confirmKind = iota
	confirmKindReviewedUpdate
	confirmKindApplyProfile
	confirmKindRestore
	confirmKindAdoptAll
	confirmKindDeleteProfile
	confirmKindBatch
	confirmKindInstall
	confirmKindDeleteSource
)

func (m Tab) updateConfirm(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	var res components.Result
	m.confirm, res = m.confirm.Update(msg, m.width, m.height)
	switch res {
	case components.Yes:
		m.mode = skModeList
		switch m.ckind {
		case confirmKindDeleteSource:
			r, svc := *m.sources.edit, m.svc
			m.mode = skModeSourceBusy
			return m, tea.Batch(m.beginSpin("removing source descriptor…"), func() tea.Msg { return sourceSavedMsg{svc.saveRepository(r, &r, true)} })
		case confirmKindInstall:
			p, svc := m.picker, m.svc
			chosen := p.chosen()
			m.mode = skModeInstallBusy
			m.picker.cleanup = ""
			return m, tea.Batch(m.beginSpin("installing and activating selected skills…"), func() tea.Msg {
				if p.cleanup != "" {
					defer os.RemoveAll(p.cleanup)
				}
				return installFlowDoneMsg{out: svc.installWithTargets(chosen, p.origin, p.agents, p.ids)}
			})
		case confirmKindBatch:
			plan, svc := m.batch.plan, m.svc
			m.mode = skModeBatchBusy
			return m, tea.Batch(m.beginSpin("applying selected skill actions…"), func() tea.Msg {
				return skillBatchMsg{plan: plan, result: svc.runBatch(plan, nil)}
			})
		case confirmKindReviewedUpdate:
			return m.applyUpdateReview()
		case confirmKindApplyProfile:
			name := m.pendingProfile
			svc, agents := m.svc, m.agents
			return m, func() tea.Msg {
				return bulkMsg{name: name, profile: true, result: svc.ApplyProfileResult(name, agents, nil)}
			}
		case confirmKindDeleteProfile:
			name, svc := m.pendingProfile, m.svc
			m.mode = skModeProfiles
			return m, func() tea.Msg {
				return profileDeleteMsg{name: name, err: svc.DeleteProfile(name)}
			}
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
		if m.ckind == confirmKindReviewedUpdate {
			m.mode = skModeUpdateReview
		}
		if m.ckind == confirmKindDeleteSource {
			m.mode = skModeSources
		}
		if m.ckind == confirmKindInstall {
			m.mode = skModePick
		}
		if m.ckind == confirmKindBatch {
			m.mode = skModeBatch
		}
		if m.ckind == confirmKindDeleteProfile {
			m.mode = skModeProfiles
		}
		if m.ckind == confirmKindRestore {
			m.mode = skModeBackup
		}
	}
	return m, nil
}
