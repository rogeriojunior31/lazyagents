package skills

import (
	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"strings"
)

func (m Tab) discardInput(back skMode) (Tab, tea.Cmd) {
	dirty := strings.TrimSpace(m.input.Value()) != ""
	if m.mode == skModeSourceForm {
		dirty = m.sourceDraft() != m.sources.initial
	}
	if dirty {
		c := components.NewConfirm("Discard unsaved changes?\nChoose No to keep the current field, focus and draft. Nothing is written.")
		m.draftDiscard = &c
		m.discardBack = back
		return m, nil
	}
	if m.mode == skModeSourceForm {
		m.sources.err = ""
		m.sources.savingForm = false
	}
	m.mode = back
	m.input.Blur()
	return m, nil
}
