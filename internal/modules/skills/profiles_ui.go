package skills

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

func (m Tab) loadProfilesCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		names, err := svc.ListProfiles()
		return profilesLoadMsg{names: names, err: err}
	}
}

func (m Tab) updateProfiles(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		return m, nil
	case "up", "k":
		if m.profileCursor > 0 {
			m.profileCursor--
		}
		return m, nil
	case "down", "j":
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
		return m, nil
	case "enter":
		if len(m.profileNames) == 0 {
			return m, nil
		}
		name := m.profileNames[m.profileCursor]
		svc, agents := m.svc, m.agents
		return m, func() tea.Msg {
			changes, err := svc.DiffProfile(name, agents)
			if err != nil {
				return profileDiffMsg{err: err}
			}
			return profileDiffMsg{name: name, changes: changes}
		}
	case "s":
		m.mode = skModeProfileName
		m.input.Placeholder = "nome do perfil (ex: trabalho)"
		m.input.SetValue("")
		return m, m.input.Focus()
	}
	return m, nil
}

func (m Tab) updateProfileName(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeProfiles
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}
		m.input.Blur()
		spec := BuildProfileSpec(m.skills, m.agents)
		svc := m.svc
		return m, func() tea.Msg {
			err := svc.SaveProfile(name, spec)
			return profileSaveMsg{name: name, err: err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Tab) profilesView() string {
	w := m.width
	if w > 72 {
		w = 72
	}
	var b strings.Builder
	if len(m.profileNames) == 0 {
		b.WriteString(kit.StHint.Render("Nenhum perfil salvo.") + "\n\n")
	} else {
		start, end := kit.Window(m.profileCursor, len(m.profileNames), m.height-8)
		for i := start; i < end; i++ {
			name := m.profileNames[i]
			if i == m.profileCursor {
				b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Render("› ") + kit.StText.Render(name) + "\n")
			} else {
				b.WriteString("  " + kit.StHint.Render(name) + "\n")
			}
		}
		if end < len(m.profileNames) {
			b.WriteString(kit.StHint.Render(fmt.Sprintf("… mais %d", len(m.profileNames)-end)) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(components.Keycap("enter") + kit.StHint.Render(" aplica  ") +
		components.Keycap("s") + kit.StHint.Render(" salva estado atual  ") +
		components.Keycap("esc") + kit.StHint.Render(" volta"))
	panel := components.Panel{Title: "Perfis de skills", Focused: true, Width: w}.Render(strings.TrimRight(b.String(), "\n"))
	if m.toast != "" {
		return lipgloss.JoinVertical(lipgloss.Left, panel, "", m.toastLine())
	}
	return panel
}

// --- module.Module ---
