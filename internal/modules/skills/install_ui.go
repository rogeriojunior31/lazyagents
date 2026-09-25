package skills

import (
	"fmt"

	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Tab) updateInstall(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		src := strings.TrimSpace(m.input.Value())
		if src == "" {
			return m, nil
		}
		m.input.Blur()
		spin := m.beginSpin(fmt.Sprintf("looking for skills in %s…", src))
		svc := m.svc
		return m, tea.Batch(spin, func() tea.Msg {
			found, origin, cleanup, err := svc.Discover(src)
			return discoverMsg{found: found, origin: origin, cleanup: cleanup, err: err}
		})
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Tab) updatePick(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		if m.picker.cleanup != "" {
			os.RemoveAll(m.picker.cleanup)
		}
		return m, nil
	case "enter":
		chosen := m.picker.chosen()
		if len(chosen) == 0 {
			m.setToast("no skill selected (space marks)", true)
			return m, nil
		}
		svc, origin, cleanup := m.svc, m.picker.origin, m.picker.cleanup
		return m, func() tea.Msg {
			names, err := svc.Install(chosen, origin)
			if cleanup != "" {
				os.RemoveAll(cleanup)
			}
			return installDoneMsg{names: names, err: err}
		}
	default:
		m.picker = m.picker.update(msg, m.width, m.height)
		return m, nil
	}
}

func (m Tab) updateRegistrySearch(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		term := strings.TrimSpace(m.input.Value())
		if term == "" {
			return m, nil
		}
		m.input.Blur()
		spin := m.beginSpin(fmt.Sprintf("searching GitHub for %q…", term))
		svc := m.svc
		return m, tea.Batch(spin, func() tea.Msg {
			results, err := svc.SearchRegistry(term)
			return registrySearchMsg{results: results, err: err}
		})
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Tab) updateRegistryPick(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		return m, nil
	case "enter":
		sel, ok := m.regPicker.selected()
		if !ok {
			return m, nil
		}
		m.mode = skModeList
		spin := m.beginSpin(fmt.Sprintf("looking for skills in %s…", sel.Repo))
		svc := m.svc
		return m, tea.Batch(spin, func() tea.Msg {
			found, origin, cleanup, err := svc.Discover(sel.Repo)
			return discoverMsg{found: found, origin: origin, cleanup: cleanup, err: err}
		})
	default:
		m.regPicker.update(msg)
		return m, nil
	}
}
