package recovery

import (
	"fmt"

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
	history          *historyView
	svc              *Service
	agentNames       map[string]string
	showUnavailable  bool
	inspection       *Entry
	detailOff        int
	entries, matches []Entry
	cursor           int
	input            textinput.Model
	filtering, busy  bool
	loadErr          bool
	preview          *Preview
	confirm          *components.Confirm
	vp               viewport.Model
	width, height    int
	notice           string
	noticeErr        bool
}

func newTab(svc *Service) *Tab {
	input := components.NewInput()
	input.Prompt = "/ "
	input.Placeholder = "Agent, file, operation or YYYY-MM-DD"
	return &Tab{svc: svc, input: input, vp: viewport.New()}
}

func (m *Tab) ID() string    { return "recovery" }
func (m *Tab) Title() string { return "Recovery" }
func (m *Tab) Count() int    { return len(m.entries) }
func (m *Tab) Init() tea.Cmd { return nil }
func (m *Tab) Capturing() bool {
	return m.history != nil || m.filtering || m.preview != nil || m.inspection != nil || m.confirm != nil || m.busy
}
func (m *Tab) ClearToast() { m.notice = "" }
func (m *Tab) Help() []module.HelpGroup {
	return []module.HelpGroup{
		{Title: "Config backups", Keys: [][2]string{{"↑/↓ · j/k", "select config backup"}, {"/", "filter by agent, file, operation or local date"}, {"enter", "preview an available backup or inspect why recovery is unavailable"}, {"l", "switch between ready-to-preview and all backups"}, {"r", "reload backups"}, {"shift+↑/↓ · ctrl+u/d", "scroll backup details"}}},
		{Title: "Operation history", Keys: [][2]string{{"h", "open recent local write attempts"}, {"enter", "in history: inspect outcomes and verify related backups"}, {"r", "in history: reload; in details: review current state in the module"}, {"[ · ] · b", "in history details: choose and freshly preview a backup"}, {"c", "in history: confirm clearing history while keeping backups"}, {"esc", "back without repeating writes"}}},
		{Title: "Restore preview", Keys: [][2]string{{"↑/↓ · pgup/pgdn", "read the full redacted diff"}, {"enter", "confirm replacement of this whole file"}, {"esc", "back without restoring"}}},
	}
}

func (m *Tab) loadCmd() tea.Cmd {
	svc := m.svc
	m.busy = true
	return func() tea.Msg { entries, err := svc.List(); return loadedMsg{entries, err} }
}

func (m *Tab) layoutPreview() {
	m.vp.SetWidth(max(1, m.width-4))
	m.vp.SetHeight(max(1, m.height-6))
	if m.inspection != nil {
		m.vp.SetContent(lipgloss.NewStyle().Width(max(1, m.width-4)).Render(m.entryDetails(*m.inspection)))
	} else if m.preview != nil {
		m.vp.SetContent(lipgloss.NewStyle().Width(max(1, m.width-4)).Render(previewText(*m.preview)))
	}
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	if detected, ok := msg.(events.AgentsDetected); ok {
		m.agentNames = map[string]string{}
		for _, ag := range detected.Agents {
			m.agentNames[ag.ID] = ag.Name
		}
		return nil
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if _, ok := msg.(openHistoryMsg); ok {
		if m.Capturing() || m.busy {
			return nil
		}
		m.history = &historyView{}
		return m.loadHistory()
	}
	if p, ok := msg.(historyPreviewMsg); ok {
		if m.history != p.view {
			return nil
		}
		m.history = nil
		m.preview = &p.preview
		m.layoutPreview()
		m.vp.GotoTop()
		return nil
	}
	if m.history != nil {
		return m.historyUpdate(msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layoutPreview()
	case events.TabActivated:
		if msg.ID == m.ID() && !m.busy {
			return m.loadCmd()
		}
	case events.Reload:
		return m.loadCmd()
	case loadedMsg:
		failedBefore := m.loadErr
		m.busy = false
		m.loadErr = msg.err != nil
		if msg.err != nil {
			m.notice = msg.err.Error()
			m.noticeErr = true
			return nil
		}
		if failedBefore && m.noticeErr {
			m.notice = ""
			m.noticeErr = false
		}
		selected := ""
		if len(m.matches) > 0 {
			selected = m.matches[m.cursor].ID
		}
		m.entries = msg.entries
		m.refilter()
		for i, e := range m.matches {
			if e.ID == selected {
				m.cursor = i
				break
			}
		}
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
		if m.inspection != nil {
			switch msg.String() {
			case "esc", "q":
				m.inspection = nil
			default:
				var cmd tea.Cmd
				m.vp, cmd = m.vp.Update(msg)
				return cmd
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
		if kit.DetailScroll[msg.String()] {
			vp := m.detailViewport(m.split())
			vp, _ = vp.Update(kit.DetailScrollMsg(msg))
			m.detailOff = vp.YOffset()
			return nil
		}
		switch msg.String() {
		case "h":
			m.history = &historyView{}
			return m.loadHistory()
		case "l":
			m.showUnavailable = !m.showUnavailable
			m.cursor, m.detailOff = 0, 0
			m.refilter()
		case "/":
			m.filtering = true
			return m.input.Focus()
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
			m.detailOff = 0
		case "down", "j":
			m.cursor = min(max(0, len(m.matches)-1), m.cursor+1)
			m.detailOff = 0
		case "r":
			m.notice = ""
			return m.loadCmd()
		case "enter":
			if len(m.matches) == 0 {
				return nil
			}
			entry := m.matches[m.cursor]
			if !entry.Restorable {
				m.inspection = &entry
				m.layoutPreview()
				m.vp.GotoTop()
				return nil
			}
			id, svc := entry.ID, m.svc
			m.busy = true
			return func() tea.Msg { p, err := svc.Preview(id); return previewMsg{p, err} }
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || m.busy || m.filtering || m.preview != nil || m.inspection != nil || m.confirm != nil {
			return nil
		}
		sp := m.split()
		y := msg.Y - lipgloss.Height(m.inventoryHeader()) - 2
		start, end := kit.Window(m.cursor, len(m.matches), max(1, sp.ListH-3))
		if msg.X >= 0 && msg.X < sp.ListW && y >= 0 && start+y < end {
			m.cursor, m.detailOff = start+y, 0
		}
	case tea.MouseWheelMsg:
		if m.busy {
			return nil
		}
		if (m.preview != nil || m.inspection != nil) && m.confirm == nil {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return cmd
		}
		if m.confirm == nil && !m.filtering {
			sp := m.split()
			if sp.DetailH > 0 && (sp.Side && msg.X >= sp.ListW || !sp.Side && msg.Y >= lipgloss.Height(m.inventoryHeader())+sp.ListH) {
				vp := m.detailViewport(sp)
				vp, _ = vp.Update(msg)
				m.detailOff = vp.YOffset()
			} else {
				if msg.Button == tea.MouseWheelUp {
					m.cursor = max(0, m.cursor-1)
				}
				if msg.Button == tea.MouseWheelDown {
					m.cursor = min(max(0, len(m.matches)-1), m.cursor+1)
				}
				m.detailOff = 0
			}
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
			m.cursor, m.detailOff = 0, 0
			m.refilter()
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.cursor, m.detailOff = 0, 0
	m.refilter()
	return cmd
}

func (m *Tab) View() string {
	if m.history != nil {
		return m.historyRender()
	}
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.inspection != nil {
		body := m.vp.View() + "\n\n" + kit.Hints(m.width-4, [2]string{"esc", "back"})
		return components.Panel{Title: "Manual recovery only", Focused: true, Width: m.width}.Render(body)
	}
	if m.preview != nil {
		body := m.vp.View() + "\n\n" + kit.Hints(m.width-4, [2]string{"enter", "restore"}, [2]string{"esc", "back"}) + "\n" + components.Toast(m.notice, m.noticeErr)
		return components.Panel{Title: "Review changes before restoring", Focused: true, Width: m.width}.Render(body)
	}
	return m.inventoryView()
}

func (m *Tab) refilter() {
	entries := []Entry{}
	for _, e := range m.entries {
		if m.showUnavailable || e.Restorable {
			entries = append(entries, e)
		}
	}
	m.matches = Filter(entries, m.input.Value())
	m.cursor = min(m.cursor, max(0, len(m.matches)-1))
}
