package recovery

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

type loadedMsg struct {
	entries []Entry
	err     error
}
type previewMsg struct {
	preview Preview
	err     error
}
type restoredMsg struct {
	agentID string
	err     error
}

type Tab struct {
	svc                     *Service
	entries, matches        []Entry
	cursor                  int
	input                   textinput.Model
	filtering, loaded, busy bool
	preview                 *Preview
	confirm                 *components.Confirm
	vp                      viewport.Model
	width, height           int
	notice                  string
	noticeErr               bool
}

func newTab(svc *Service) *Tab {
	input := components.NewInput()
	input.Prompt = "/ "
	input.Placeholder = "Agent, file, operation or YYYY-MM-DD"
	return &Tab{svc: svc, input: input, vp: viewport.New()}
}

func (m *Tab) ID() string      { return "recovery" }
func (m *Tab) Title() string   { return "Recovery" }
func (m *Tab) Count() int      { return len(m.entries) }
func (m *Tab) Init() tea.Cmd   { return nil }
func (m *Tab) Capturing() bool { return m.filtering || m.preview != nil || m.confirm != nil || m.busy }
func (m *Tab) ClearToast()     { m.notice = "" }
func (m *Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Config backups", Keys: [][2]string{{"↑/↓ · j/k", "select config backup"}, {"/", "filter by agent, file, operation or local date"}, {"enter", "verify and preview the selected backup"}, {"r", "reload backups"}}},
		{Title: "Restore preview", Keys: [][2]string{{"↑/↓ · pgup/pgdn", "read the full redacted diff"}, {"enter", "confirm replacement of this whole file"}, {"esc", "back without restoring"}}},
	}
}

func (m *Tab) loadCmd() tea.Cmd {
	svc := m.svc
	m.loaded, m.busy = true, true
	return func() tea.Msg { entries, err := svc.List(); return loadedMsg{entries, err} }
}

func (m *Tab) layoutPreview() {
	m.vp.SetWidth(max(1, m.width-4))
	m.vp.SetHeight(max(1, m.height-6))
	if m.preview != nil {
		m.vp.SetContent(lipgloss.NewStyle().Width(max(1, m.width-4)).Render(previewText(*m.preview)))
	}
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layoutPreview()
	case events.TabActivated:
		if msg.ID == m.ID() && !m.loaded {
			return m.loadCmd()
		}
	case events.Reload:
		return m.loadCmd()
	case loadedMsg:
		m.busy = false
		if msg.err != nil {
			m.notice = msg.err.Error()
			m.noticeErr = true
			return nil
		}
		m.entries, m.matches = msg.entries, Filter(msg.entries, m.input.Value())
		m.cursor = min(m.cursor, max(0, len(m.matches)-1))
	case previewMsg:
		m.busy = false
		if msg.err != nil {
			m.notice = msg.err.Error()
			m.noticeErr = true
			return nil
		}
		m.preview, m.notice = &msg.preview, ""
		m.layoutPreview()
		m.vp.GotoTop()
	case restoredMsg:
		m.busy, m.preview, m.confirm = false, nil, nil
		m.notice = "Config restored; other files and provider ownership records are unchanged"
		m.noticeErr = msg.err != nil
		if msg.err != nil {
			m.notice = msg.err.Error()
		}
		return tea.Batch(m.loadCmd(), func() tea.Msg { return events.AgentConfigChanged{AgentID: msg.agentID} })
	case tea.KeyPressMsg:
		if m.busy {
			return nil
		}
		if m.filtering {
			return m.filterMsg(msg)
		}
		if m.confirm != nil {
			confirm, result := m.confirm.Update(msg, m.width, m.height)
			m.confirm = &confirm
			if result == components.No {
				m.confirm = nil
			}
			if result == components.Yes {
				p, svc := *m.preview, m.svc
				m.confirm, m.busy = nil, true
				m.notice = "Restoring config..."
				m.noticeErr = false
				return func() tea.Msg { return restoredMsg{p.Agent, svc.Restore(p.ID, p.Review)} }
			}
			return nil
		}
		if m.preview != nil {
			switch msg.String() {
			case "esc", "q":
				m.preview = nil
			case "enter":
				confirm := components.NewConfirm(fmt.Sprintf("Replace the whole config file %s with backup %s?\nThis includes unrelated edits shown in the preview.\nCurrent files are backed up; changed files require a fresh preview.", m.preview.Destination, m.preview.ID))
				m.confirm = &confirm
			default:
				var cmd tea.Cmd
				m.vp, cmd = m.vp.Update(msg)
				return cmd
			}
			return nil
		}
		switch msg.String() {
		case "/":
			m.filtering = true
			return m.input.Focus()
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(max(0, len(m.matches)-1), m.cursor+1)
		case "r":
			m.notice = ""
			return m.loadCmd()
		case "enter":
			if len(m.matches) == 0 {
				return nil
			}
			entry := m.matches[m.cursor]
			if !entry.Restorable {
				m.notice = entry.Reason
				m.noticeErr = true
				return nil
			}
			id, svc := entry.ID, m.svc
			m.busy = true
			return func() tea.Msg { p, err := svc.Preview(id); return previewMsg{p, err} }
		}
	case tea.MouseWheelMsg:
		if m.preview != nil && m.confirm == nil {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return cmd
		}
	default:
		if m.filtering {
			return m.filterMsg(msg)
		}
	}
	return nil
}

func (m *Tab) filterMsg(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter":
			m.filtering = false
			m.input.Blur()
			return nil
		case "esc":
			m.filtering = false
			m.input.Blur()
			m.input.SetValue("")
			m.matches, m.cursor = m.entries, 0
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.matches, m.cursor = Filter(m.entries, m.input.Value()), 0
	return cmd
}

func (m *Tab) View() string {
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	hints := kit.Hints(m.width-4, [2]string{"enter", "preview"}, [2]string{"/", "filter"}, [2]string{"?", "help"})
	if m.preview != nil {
		body := m.vp.View() + "\n\n" + kit.Hints(m.width-4, [2]string{"enter", "restore"}, [2]string{"esc", "back"}) + "\n" + components.Toast(m.notice, m.noticeErr)
		return components.Panel{Title: "Config restore preview", Focused: true, Width: m.width}.Render(body)
	}
	if m.filtering {
		hints = kit.Hints(m.width-4, [2]string{"enter", "apply filter"}, [2]string{"esc", "clear filter"})
	}
	lines := []string{components.InputView(m.input, m.width-4)}
	start, end := kit.Window(m.cursor, len(m.matches), max(1, (m.height-7)/2))
	for i := start; i < end; i++ {
		e := m.matches[i]
		mark := "  "
		if i == m.cursor {
			mark = "› "
		}
		lines = append(lines, fmt.Sprintf("%s%s  %s  %s", mark, e.Time.Local().Format("2006-01-02 15:04"), e.Agent, e.Operation), "  "+e.ID)
	}
	if len(m.matches) == 0 {
		lines = append(lines, "No config backups found. Skill archives are in Skills.")
	}
	lines = append(lines, "", hints, components.Toast(m.notice, m.noticeErr))
	return components.Panel{Title: fmt.Sprintf("Config backups (%d)", len(m.matches)), Focused: true, Width: m.width}.Render(strings.Join(lines, "\n"))
}
