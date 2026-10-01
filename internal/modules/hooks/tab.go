package hooks

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// Tab is the hooks tab: the hook × agent matrix and installing a hook into each
// CLI's live config. Like every agent-file write, it goes through a confirm that
// says what will be rewritten. Pointer semantics (module.Module).
type Tab struct {
	svc      *Service
	lib      []Hook
	problems []string
	statuses []Status

	cursor    int
	col       int // agent under the cursor (index in statuses)
	detailOff int // detail panel scroll
	// cmdMode sends keys to the entry's command list (enter) to toggle them one by
	// one; cmdCursor is the position in commandOrder.
	reader    *hookReader
	cmdMode   bool
	cmdCursor int
	confirm   *components.Confirm
	action    func() tea.Msg

	loaded, loading bool
	width, height   int
	toast           string
	toastErr        bool
}

func newTab(svc *Service) Tab { return Tab{svc: svc} }

// Init reads only the library so the pill counter is right before the tab
// opens; agent state (which detects the CLIs) waits for activation.
func (m Tab) Init() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		lib, _ := svc.Library()
		return libraryMsg{lib: lib}
	}
}

func (m *Tab) ID() string     { return "hooks" }
func (m *Tab) Title() string  { return "Hooks" }
func (m Tab) Count() int      { return len(m.lib) }
func (m Tab) Capturing() bool { return m.confirm != nil || m.cmdMode || m.reader != nil }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd reads the library and what each agent has installed, off the render
// thread: Status detects agents and reads live files.
func (m *Tab) loadCmd() tea.Cmd {
	svc := m.svc
	m.loading = true
	return func() tea.Msg {
		lib, problems := svc.Library()
		return loadedMsg{lib: lib, problems: problems, statuses: svc.Status()}
	}
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.cmdMode {
			m.detailOff = 0
		}

	case events.TabActivated:
		if msg.ID == m.ID() && !m.loaded && !m.loading {
			m.loaded = true
			return m.loadCmd()
		}

	case events.Reload:
		m.loaded = true
		return m.loadCmd()

	case libraryMsg:
		if !m.loaded { // the full load already brought the library
			m.lib = msg.lib
		}

	case loadedMsg:
		m.loading = false
		m.lib, m.problems, m.statuses = msg.lib, msg.problems, msg.statuses
		m.cursor = min(m.cursor, max(0, len(m.lib)-1))
		m.col = max(0, min(m.col, len(m.statuses)-1))
		if h, ok := m.current(); !ok || m.cmdCursor >= len(h.Hooks) {
			m.cmdMode, m.cmdCursor = false, 0
		}
		if len(msg.problems) > 0 {
			m.toast, m.toastErr = msg.problems[0], true
		}

	case documentsMsg:
		if m.reader != msg.reader {
			return nil
		}
		m.reader.docs = msg.docs
		if len(msg.docs) > 1 {
			m.reader.selected = 1
		}
		if msg.err != nil {
			m.reader.notice = msg.err.Error()
		} else if len(msg.docs) == 1 {
			m.reader.notice = "No literal script found; ←→ switches files"
		} else {
			m.reader.notice = "←→ switches command and scripts · PgUp/PgDn scrolls"
		}
	case editPreparedMsg:
		return m.preparedEdit(msg)
	case hookEditedMsg:
		return m.finishEdit(msg)
	case editSavedMsg:
		if m.reader != msg.reader {
			return nil
		}
		if msg.err != nil {
			m.reader.notice = msg.err.Error()
			return m.loadCmd()
		}
		m.reader.hook, m.reader.docs = msg.hook, msg.docs
		m.reader.selected = min(m.reader.selected, len(msg.docs)-1)
		m.reader.notice = "Saved with backup"
		return m.loadCmd()
	case doneMsg:
		m.toast, m.toastErr = msg.text, msg.err
		return m.loadCmd()

	case tea.KeyPressMsg:
		return m.key(msg)

	case tea.MouseClickMsg:
		if m.confirm != nil {
			return nil
		}
		if m.cmdMode || m.reader != nil {
			return nil
		}
		sp := m.split()
		if msg.Button != tea.MouseLeft || msg.X >= sp.ListW || msg.Y >= sp.ListH {
			return nil // detail: only the wheel acts on it
		}
		start, end := kit.Window(m.cursor, len(m.lib), max(1, sp.ListH-hooksTop))
		if i := start + msg.Y - hooksTop; msg.Y >= hooksTop && i < end {
			m.move(i - m.cursor)
			if c := kit.ColumnAt(sp.ListW, m.tableCols(sp.ListW), msg.X) - colAgents; c >= 0 && c < len(m.statuses) {
				m.col = c // clicking a cell picks the agent; space toggles
			}
		}

	case tea.MouseWheelMsg:
		if m.confirm != nil {
			c, _ := m.confirm.Update(msg, m.width, m.height)
			m.confirm = &c
			return nil
		}
		if m.reader != nil {
			return m.updateReader(msg)
		}
		if msg.Y < 0 || msg.Y >= m.bodyHeight() || msg.X < 0 || msg.X >= m.width {
			return nil
		}
		delta, key := 0, ""
		switch msg.Button {
		case tea.MouseWheelUp:
			delta, key = -1, "up"
		case tea.MouseWheelDown:
			delta, key = 1, "down"
		default:
			return nil
		}
		if m.cmdMode {
			return m.cmdKey(key)
		}
		if sp := m.split(); !sp.Side && msg.Y >= sp.ListH || sp.Side && msg.X >= sp.ListW+2 {
			m.detailOff = max(0, min(m.maxDetailOff(), m.detailOff+delta))
		} else {
			m.move(delta)
		}
	}
	return nil
}

func (m *Tab) key(msg tea.KeyPressMsg) tea.Cmd {
	if m.confirm != nil {
		c, res := m.confirm.Update(msg, m.width, m.height)
		m.confirm = &c
		switch res {
		case components.Yes:
			action := m.action
			m.confirm, m.action = nil, nil
			m.toast, m.toastErr = "applying…", false
			return func() tea.Msg { return action() }
		case components.No:
			m.confirm, m.action = nil, nil
			m.toast = ""
		}
		return nil
	}

	if m.reader != nil {
		return m.updateReader(msg)
	}
	key := msg.String()
	// Paging also works during selection, without changing the target.
	step := max(1, m.detailRows()-1)
	switch key {
	case "v":
		return m.openReader()
	case "pgdown", "ctrl+d":
		m.detailOff = min(m.maxDetailOff(), m.detailOff+step)
		return nil
	case "pgup", "ctrl+u":
		m.detailOff = max(0, m.detailOff-step)
		return nil
	case "shift+down":
		m.detailOff = min(m.maxDetailOff(), m.detailOff+1)
		return nil
	case "shift+up":
		m.detailOff = max(0, m.detailOff-1)
		return nil
	}
	if m.cmdMode {
		return m.cmdKey(key)
	}
	switch key {
	case "enter":
		if h, ok := m.current(); ok && len(h.Hooks) > 0 {
			m.cmdMode, m.cmdCursor = true, 0
			m.detailOff = 0
		}
	case "left", "h":
		m.col = max(0, m.col-1)
	case "right", "l":
		m.col = max(0, min(len(m.statuses)-1, m.col+1))
	case "space":
		return m.toggleAgent(m.col)
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "r":
		m.toast = ""
		return m.loadCmd()
	case "a":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask(fmt.Sprintf("Install hook %s in every agent that fires %s?\n%s\nThe config files are rewritten (with backup).", h.Name, strings.Join(h.Events(), ", "), h.Summary()), func() tea.Msg {
			return done(m.svc.Enable(h.Name, ""), fmt.Sprintf("hook %s installed in every agent", h.Name))
		})
	case "x":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask(fmt.Sprintf("Uninstall hook %s from every agent?", h.Name), func() tea.Msg {
			return done(m.svc.Disable(h.Name, ""), fmt.Sprintf("hook %s uninstalled from every agent", h.Name))
		})
	case "d":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask(fmt.Sprintf("Delete hook %s from the library? (agents where it is installed keep it)", h.Name), func() tea.Msg {
			return done(m.svc.Delete(h.Name), fmt.Sprintf("hook %s deleted from the library", h.Name))
		})
	}
	return nil
}

// cmdKey handles keys in the entry's command list.
func (m *Tab) cmdKey(key string) tea.Cmd {
	h, ok := m.current()
	if !ok {
		m.cmdMode = false
		return nil
	}
	switch key {
	case "esc", "q", "left", "h":
		m.cmdMode = false
	case "up", "k":
		m.cmdCursor = max(0, m.cmdCursor-1)
		m.detailOff = 0
	case "down", "j":
		m.cmdCursor = min(len(h.Hooks)-1, m.cmdCursor+1)
		m.detailOff = 0
	case "space", "enter":
		return m.toggleCommand(h)
	}
	return nil
}

// toggleCommand turns the command under the cursor on or off. If the entry is
// installed nowhere only the library changes, with no confirm; otherwise the
// agent file is rewritten.
func (m *Tab) toggleCommand(h Hook) tea.Cmd {
	order := commandOrder(h)
	if m.cmdCursor >= len(order) {
		return nil
	}
	i := order[m.cmdCursor]
	c := h.Hooks[i]
	on := h.IsOff(i)
	result, question := "%s · %s disabled", "Disable %s · %s in %s?\n%s\nRewrites %s (with backup)."
	if on {
		result, question = "%s · %s enabled", "Enable %s · %s in %s?\n%s\nRewrites %s (with backup)."
	}
	svc, name := m.svc, h.Name
	action := func() tea.Msg {
		return done(svc.SetCommand(name, i, on), fmt.Sprintf(result, name, commandLabel(c)))
	}
	var where []string
	for _, st := range m.statuses {
		if enabledIn(st, name) || partialIn(st, name) {
			where = append(where, st.AgentName+" ("+core.Tilde(st.File, m.svc.home)+")")
		}
	}
	if len(where) == 0 {
		return func() tea.Msg { return action() }
	}
	return m.ask(fmt.Sprintf(question,
		c.Event, commandLabel(c), name, displayCommand(c.Command), strings.Join(where, ", ")), action)
}

// toggleAgent installs the hook in agent i, or removes it if already there.
func (m *Tab) toggleAgent(i int) tea.Cmd {
	h, ok := m.current()
	if !ok || i >= len(m.statuses) {
		return nil
	}
	st := m.statuses[i]
	if !st.Installed {
		m.toast, m.toastErr = st.AgentID+" is not installed", true
		return nil
	}
	if enabledIn(st, h.Name) || partialIn(st, h.Name) {
		return m.ask(fmt.Sprintf("Uninstall hook %s from %s?\nRewrites %s (with backup).", h.Name, st.AgentName, core.Tilde(st.File, m.svc.home)), func() tea.Msg {
			return done(m.svc.Disable(h.Name, st.AgentID), fmt.Sprintf("hook %s uninstalled from %s", h.Name, st.AgentID))
		})
	}
	question := fmt.Sprintf("Install hook %s in %s?\n%s → %s\nRewrites %s (with backup).",
		h.Name, st.AgentName, strings.Join(h.Events(), ", "), h.Summary(), core.Tilde(st.File, m.svc.home))
	if h.Imported() {
		question += fmt.Sprintf("\nImported from %s (made for Claude Code).", h.Source)
	}
	if st.Note != "" {
		question += "\n" + st.Note
	}
	return m.ask(question, func() tea.Msg {
		return done(m.svc.Enable(h.Name, st.AgentID), fmt.Sprintf("hook %s installed in %s", h.Name, st.AgentID))
	})
}

func (m *Tab) ask(question string, action func() tea.Msg) tea.Cmd {
	c := components.NewConfirm(question)
	m.confirm, m.action = &c, action
	return nil
}

func done(err error, ok string) doneMsg {
	if err != nil {
		return doneMsg{text: err.Error(), err: true}
	}
	return doneMsg{text: ok}
}

func enabledIn(st Status, name string) bool { return contains(st.Enabled, name) }

func partialIn(st Status, name string) bool { return contains(st.Partial, name) }

func contains(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

func (m *Tab) current() (Hook, bool) {
	if m.cursor < 0 || m.cursor >= len(m.lib) {
		return Hook{}, false
	}
	return m.lib[m.cursor], true
}

func (m *Tab) move(d int) {
	if len(m.lib) == 0 {
		return
	}
	if c := max(0, min(len(m.lib)-1, m.cursor+d)); c != m.cursor {
		m.cursor, m.detailOff, m.cmdCursor = c, 0, 0
	}
}
