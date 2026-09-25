package sessions

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// toggleSelectGroup marca/desmarca todas as sessões de um grupo de uma vez:
// se todas já estão marcadas, desmarca; senão marca as que faltam.
func (m *Tab) toggleSelectGroup(ids []string) tea.Cmd {
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	allSelected := true
	for _, id := range ids {
		if !m.selected[id] {
			allSelected = false
			break
		}
	}
	for _, id := range ids {
		if allSelected {
			delete(m.selected, id)
		} else {
			m.selected[id] = true
		}
	}
	return m.applyItems()
}

// toggleSelect alterna a seleção de uma sessão pelo ID.
func (m *Tab) toggleSelect(id string) tea.Cmd {
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	if m.selected[id] {
		delete(m.selected, id)
	} else {
		m.selected[id] = true
	}
	return m.applyItems()
}

// selectedSessions devolve as sessões marcadas para deleção.
func (m Tab) selectedSessions() []agent.Session {
	var out []agent.Session
	for _, s := range m.sessions {
		if m.selected[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// deleteCmd executa a deleção das sessões em background e reporta o resultado.
func (m Tab) deleteCmd(targets []agent.Session) tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		var deleted, failed int
		var errs []error
		for _, s := range targets {
			if err := svc.DeleteSession(s); err != nil {
				failed++
				errs = append(errs, fmt.Errorf("%s: %w", kit.Truncate(s.Title, 40), err))
			} else {
				deleted++
			}
		}
		return deleteSessionsMsg{deleted: deleted, failed: failed, errs: errs}
	}
}

// Guarda os alvos mostrados: uma recarga assíncrona não muda a decisão aberta.
func (m *Tab) askDelete() {
	m.deleteTargets = m.selectedSessions()
	var b strings.Builder
	fmt.Fprintf(&b, "Delete %d session(s)?\nBackup in %s\n", len(m.deleteTargets), core.Tilde(filepath.Join(m.svc.BackupsDir(), "sessions"), m.home))
	for _, s := range m.deleteTargets {
		fmt.Fprintf(&b, "\n• %s · %s", s.AgentName, s.Title)
	}
	c := components.NewConfirm(b.String())
	m.confirm = &c
}
