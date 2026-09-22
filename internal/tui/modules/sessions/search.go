package sessions

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// updateSearch trata o input de busca full-text nos transcripts (M8.A3).
func (m Sessions) updateSearch(msg tea.KeyPressMsg) (Sessions, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = sessModeList
		m.searchInput.Blur()
		return m, nil
	case "enter":
		q := strings.TrimSpace(m.searchInput.Value())
		m.searchInput.Blur()
		m.mode = sessModeList
		if q == "" {
			return m, nil
		}
		svc, sessions := m.svc, m.sessions
		spin := m.beginSpin("buscando \"" + q + "\" nos transcripts…")
		return m, tea.Batch(spin, func() tea.Msg {
			matches, err := svc.Search(sessions, q)
			return searchDoneMsg{query: q, matches: matches, err: err}
		})
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}
