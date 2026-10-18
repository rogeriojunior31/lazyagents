package recovery

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

type openHistoryMsg struct{}
type historyView struct {
	entries []core.HistoryEntry
	cursor  int
	detail  *HistoryInspection
	backup  int
	busy    bool
	loadErr bool
	notice  string
	confirm *components.Confirm
	vp      viewport.Model
}
type historyLoadedMsg struct {
	view    *historyView
	entries []core.HistoryEntry
	err     error
}
type historyInspectedMsg struct {
	view   *historyView
	detail HistoryInspection
	err    error
}
type historyClearedMsg struct {
	view *historyView
	err  error
}

func (m *Tab) Commands() []module.Command {
	return []module.Command{{Name: "history", Desc: "Inspect recent write attempts; never replay saved commands", Msg: openHistoryMsg{}}}
}

func (m *Tab) loadHistory() tea.Cmd {
	h := m.history
	h.busy = true
	svc := m.svc
	return func() tea.Msg { entries, err := svc.historyEntries(); return historyLoadedMsg{h, entries, err} }
}

func (m *Tab) historyUpdate(msg tea.Msg) tea.Cmd {
	h := m.history
	switch msg := msg.(type) {
	case historyLoadedMsg:
		if msg.view != h {
			return nil
		}
		h.busy = false
		h.loadErr = msg.err != nil
		if msg.err != nil {
			h.notice = msg.err.Error()
			return nil
		}
		selected := ""
		if h.cursor < len(h.entries) {
			selected = h.entries[h.cursor].ID
		}
		h.entries = msg.entries
		h.cursor = min(h.cursor, max(0, len(h.entries)-1))
		for i, e := range h.entries {
			if e.ID == selected {
				h.cursor = i
				break
			}
		}
		h.notice = ""
	case historyInspectedMsg:
		if msg.view != h {
			return nil
		}
		h.busy = false
		if msg.err != nil {
			h.notice = msg.err.Error()
			return nil
		}
		h.detail = &msg.detail
		h.backup = 0
		h.vp = viewport.New()
		h.notice = ""
	case historyClearedMsg:
		if msg.view != h {
			return nil
		}
		h.busy = false
		if msg.err != nil {
			h.notice = msg.err.Error()
			return nil
		}
		h.detail = nil
		h.notice = "History cleared; backups kept"
		h.loadErr = false
		h.entries = nil
		h.cursor = 0
	case events.TabActivated:
		if msg.ID == m.ID() && !h.busy {
			h.notice = ""
			if h.detail == nil {
				return m.loadHistory()
			}
		}
	case events.Reload:
		if !h.busy {
			h.detail = nil
			return m.loadHistory()
		}
	case tea.KeyPressMsg:
		if h.busy {
			return nil
		}
		if h.confirm != nil {
			confirm, answer := h.confirm.Update(msg, m.width, m.height)
			h.confirm = &confirm
			if answer == components.No {
				h.confirm = nil
			}
			if answer == components.Yes {
				h.confirm = nil
				h.busy = true
				svc := m.svc
				return func() tea.Msg { return historyClearedMsg{h, svc.clearHistory()} }
			}
			return nil
		}
		if h.detail != nil {
			switch msg.String() {
			case "esc", "q":
				h.detail = nil
				h.notice = ""
				return nil
			case "r":
				id := h.detail.Module
				h.notice = "Review current state in " + id + "; enable its tab in settings if hidden, or use the CLI"
				return func() tea.Msg { return events.OpenTab{ID: id} }
			case "[":
				h.backup = max(0, h.backup-1)
				return nil
			case "]":
				h.backup = min(max(0, len(h.detail.Backups)-1), h.backup+1)
				return nil
			case "b":
				if len(h.detail.Backups) == 0 {
					return nil
				}
				b := h.detail.Backups[h.backup]
				if b.Kind == "skill" {
					h.notice = "Open Skills backups (B) to inspect this archive"
					return func() tea.Msg { return events.OpenTab{ID: "skills"} }
				}
				svc := m.svc
				h.busy = true
				// Preview revalidates provenance, integrity and the current destination.
				return func() tea.Msg {
					p, err := svc.Preview(b.ID)
					if err != nil || p.Agent != b.Target {
						return historyInspectedMsg{view: h, err: fmt.Errorf("backup is unavailable for this target; reload and inspect current backups")}
					}
					return historyPreviewMsg{h, p}
				}
			}
			m.sizeHistory()
			h.vp, _ = h.vp.Update(msg)
			return nil
		}
		switch msg.String() {
		case "esc", "q", "h":
			m.history = nil
		case "up", "k":
			h.cursor = max(0, h.cursor-1)
		case "down", "j":
			h.cursor = min(max(0, len(h.entries)-1), h.cursor+1)
		case "r":
			return m.loadHistory()
		case "c":
			c := components.NewConfirm("Clear local operation history?\nAll backups and current settings are kept.")
			h.confirm = &c
		case "enter":
			if len(h.entries) == 0 {
				return nil
			}
			svc, id := m.svc, h.entries[h.cursor].ID
			h.busy = true
			return func() tea.Msg { p, err := svc.inspectHistory(id); return historyInspectedMsg{h, p, err} }
		}
	case tea.MouseWheelMsg:
		if h.busy || h.confirm != nil {
			return nil
		}
		if h.detail != nil {
			m.sizeHistory()
			h.vp, _ = h.vp.Update(msg)
		} else {
			if msg.Button == tea.MouseWheelUp {
				h.cursor = max(0, h.cursor-1)
			}
			if msg.Button == tea.MouseWheelDown {
				h.cursor = min(max(0, len(h.entries)-1), h.cursor+1)
			}
		}
	}
	return nil
}

type historyPreviewMsg struct {
	view    *historyView
	preview Preview
}

func (m *Tab) historyFooter() string {
	h := m.history
	if h.busy {
		return "Loading operation metadata…"
	}
	actions := [][2]string{{"enter", "inspect"}, {"r", "reload"}, {"c", "clear history"}, {"esc", "back"}}
	if h.detail != nil {
		actions = [][2]string{{"↑↓", "scroll"}, {"r", "review in module"}, {"esc", "back"}}
		if len(h.detail.Backups) > 0 {
			actions = append(actions, [2]string{"[ ]", "backup"}, [2]string{"b", fmt.Sprintf("preview backup %d/%d", h.backup+1, len(h.detail.Backups))})
		}
	}
	footer := kit.Hints(m.width-4, actions...)
	if m.width < 70 {
		if h.detail == nil {
			footer = kit.Hints(m.width-4, [2]string{"enter", "inspect"}, [2]string{"esc", "back"}) + "\n" + kit.Hints(m.width-4, [2]string{"r", "reload"}, [2]string{"c", "clear"})
		} else {
			footer = kit.Hints(m.width-4, [2]string{"r", "review"}, [2]string{"esc", "back"})
			if len(h.detail.Backups) > 0 {
				footer += "\n" + kit.Hints(m.width-4, [2]string{"b", "preview"}, [2]string{"[ ]", fmt.Sprintf("%d/%d", h.backup+1, len(h.detail.Backups))})
			}
		}
	}
	if h.notice != "" {
		lines := strings.Split(kit.Wrap(h.notice, max(1, m.width-4), ""), "\n")
		footer += "\n" + strings.Join(lines[:min(2, len(lines))], "\n")
	}
	return footer
}

func (m *Tab) sizeHistory() {
	h := m.history
	if h.detail == nil {
		return
	}
	h.vp.SetWidth(max(1, m.width-4))
	h.vp.SetHeight(max(1, m.height-lipgloss.Height(m.historyFooter())-3))
	offset := h.vp.YOffset()
	h.vp.SetContent(kit.Wrap(historyText(*h.detail), max(1, m.width-4), ""))
	h.vp.SetYOffset(offset)
}

func (m *Tab) historyRender() string {
	h := m.history
	if h.confirm != nil {
		return h.confirm.ViewIn(m.width, m.height)
	}
	m.sizeHistory()
	footer := m.historyFooter()
	room := max(1, m.height-lipgloss.Height(footer)-3)
	content := ""
	if h.detail != nil {
		content = h.vp.View()
	} else if h.loadErr && len(h.entries) == 0 {
		content = kit.Wrap("Operation history is unavailable. Inspect the notice below, then reload or explicitly clear this local history file.", max(1, m.width-4), "")
	} else if len(h.entries) == 0 {
		content = kit.Wrap("No recorded writes yet. Provider, hook and skill operations record their target outcomes here. Backups remain in their own inventories.", max(1, m.width-4), "")
	} else {
		start, end := kit.Window(h.cursor, len(h.entries), max(1, room/2))
		for i := start; i < end; i++ {
			e := h.entries[i]
			prefix := "  "
			if i == h.cursor {
				prefix = "› "
			}
			label := prefix + e.Time.Local().Format("01-02 15:04") + " " + e.Module + " / " + e.Action
			line := ansi.Truncate(label, max(1, m.width-4), "…")
			if i == h.cursor {
				line = kit.StOn.Render(line)
			}
			content += line + "\n" + ansi.Truncate("  "+historySummary(e), max(1, m.width-4), "…") + "\n"
		}
	}
	content = kit.Frame(content, "", room)
	body := components.Panel{Title: "Recent operations", Focused: true, Width: m.width}.Render(content + "\n" + footer)
	return kit.Frame(body, "", m.height)
}
