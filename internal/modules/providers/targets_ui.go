package providers

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type targetPicker struct {
	plan           Preparation
	selected       map[string]bool
	cursor, scroll int
	inspect        bool
	defaults       bool
	err            string
}

type preparedMsg struct {
	request uint64
	plan    Preparation
	inspect bool
}

func (m *Tab) prepareCmd(name string, ids []string) tea.Cmd {
	m.validationRequest++
	request, svc := m.validationRequest, m.svc
	m.preparing = true
	return func() tea.Msg { return preparedMsg{request: request, plan: svc.PrepareSaved(name, ids)} }
}

func (m *Tab) targetKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.targets
	switch msg.String() {
	case "esc":
		m.targets = nil
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
		p.scroll = 0
	case "down", "j":
		p.cursor = min(max(0, len(p.plan.Targets)-1), p.cursor+1)
		p.scroll = 0
	case "space":
		if !p.inspect && len(p.plan.Targets) > 0 {
			t := p.plan.Targets[p.cursor]
			if !t.Skipped {
				p.selected[t.ID] = !p.selected[t.ID]
				p.err = ""
			}
		}
	case "enter":
		if p.inspect {
			m.targets = nil
			return nil
		}
		ids := make([]string, 0)
		var lines []string
		for _, t := range p.plan.Targets {
			if !p.selected[t.ID] {
				continue
			}
			if t.Error != "" {
				p.err = "Fix or unselect incompatible agents before continuing"
				return nil
			}
			ids = append(ids, t.ID)
			lines = append(lines, "\n"+m.targetName(t.ID))
			for _, file := range t.Files {
				lines = append(lines, core.Tilde(file, m.svc.home))
			}
			lines = append(lines, t.Notes...)
		}
		if p.plan.Error != "" {
			p.err = p.plan.Error
			return nil
		}
		if len(ids) == 0 {
			p.err = "Select at least one compatible agent"
			return nil
		}
		name, svc := p.plan.Profile, m.svc
		question := fmt.Sprintf("Apply profile %s to these selected agents?\nEach config is backed up before writing.\n%s", name, strings.Join(lines, "\n"))
		if p.defaults {
			question = "Return these agents to their native/previous provider settings?\nEach config is backed up before writing.\n" + strings.Join(lines, "\n")
			return m.ask(question, func() tea.Msg { return bulkMsg{result: svc.ClearResult(ids)} })
		}
		return m.ask(question, func() tea.Msg {
			return bulkMsg{name: name, enable: true, result: svc.ApplyResult(name, ids)}
		})
	default:
		vp, _ := m.targetDetailViewport().Update(kit.DetailScrollMsg(msg))
		p.scroll = vp.YOffset()
	}
	return nil
}

func (m Tab) targetName(id string) string {
	for _, st := range m.statuses {
		if st.AgentID == id && st.AgentName != "" {
			return st.AgentName
		}
	}
	return id
}

func (m Tab) targetDetailViewport() viewport.Model {
	p := m.targets
	w, rows, footer := m.targetLayout()
	h := max(1, m.height-4-rows-lipgloss.Height(footer))
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	text := p.plan.Error
	if len(p.plan.Targets) > 0 {
		t := p.plan.Targets[p.cursor]
		lines := []string{m.targetName(t.ID)}
		if t.Error != "" {
			lines = append(lines, t.Error)
		}
		for _, file := range t.Files {
			lines = append(lines, core.Tilde(file, m.svc.home))
		}
		lines = append(lines, t.Notes...)
		if text != "" {
			lines = append(lines, text)
		}
		text = strings.Join(lines, "\n")
	}
	vp.SetContent(kit.Wrap(text, w, ""))
	vp.SetYOffset(p.scroll)
	return vp
}

func (m Tab) targetsView() string {
	p := m.targets
	panel := components.Panel{Title: "Choose agents · " + p.plan.Profile, Focused: true, Width: min(m.width, 88)}
	if p.inspect {
		panel.Title = "Check compatibility · " + p.plan.Profile
	}
	inner := panel.ContentWidth()
	_, rows, footer := m.targetLayout()
	start, end := kit.Window(p.cursor, len(p.plan.Targets), max(1, rows))
	status := "No files changed · credentials not verified"
	if p.defaults {
		status = "Select agents · native login is preserved"
	}
	lines := []string{kit.StHint.Render(ansi.Truncate(status, inner, "…"))}
	for i := start; i < end; i++ {
		t := p.plan.Targets[i]
		mark, state := "[ ]", "ready"
		if p.selected[t.ID] {
			mark = "[x]"
		}
		if t.Error != "" {
			state = "incompatible"
		}
		if t.Skipped {
			mark, state = " - ", "unavailable"
		}
		if p.inspect {
			mark = "   "
		}
		cursor := "  "
		if i == p.cursor {
			cursor = "▸ "
		}
		line := fmt.Sprintf("%s%s %s · %s", cursor, mark, m.targetName(t.ID), state)
		lines = append(lines, ansi.Truncate(line, inner, "…"))
	}
	vp := m.targetDetailViewport()
	lines = append(lines, "", vp.View())
	lines = append(lines, footer)
	return panel.Render(strings.Join(lines, "\n"))
}

func (m Tab) targetLayout() (int, int, string) {
	p := m.targets
	inner := max(1, min(m.width, 88)-4)
	note := "↑↓ agent · shift+↑↓ scroll details"
	if p.err != "" {
		note = p.err
	}
	hints := "space select · enter review · esc back"
	if p.inspect {
		hints = "esc back to form"
	}
	if m.height < 12 {
		note = "↑↓ agent · shift+↑↓ details"
		if p.err != "" {
			note = ansi.Truncate(p.err, inner, "…")
		}
		if !p.inspect {
			hints = "space select · enter · esc"
		}
	}
	footer := kit.Wrap(note, inner, "") + "\n" + kit.Wrap(hints, inner, "")
	// Keep the chosen agent, scrollable requirements and actions reachable together.
	rows := min(len(p.plan.Targets), max(1, (m.height-4-lipgloss.Height(footer))/2))
	return inner, rows, footer
}
