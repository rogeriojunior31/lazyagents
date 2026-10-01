package sessions

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type sessMode int

const (
	sessModeList   sessMode = iota
	sessModeDoc             // reading a transcript
	sessModeDir             // folder input for resume
	sessModeSearch          // full-text search input
	sessModeAlias           // alias input (key m)
)

// Tab lists every agent's sessions. enter suspends the TUI and resumes the
// session in its CLI; v opens the transcript.
type Tab struct {
	svc           *Service
	home          string
	sessions      []agent.Session
	list          list.Model
	vp            viewport.Model
	detailVP      viewport.Model
	detailID      string // resets the scroll when the selection changes
	mode          sessMode
	docTitle      string
	docSession    agent.Session // open transcript, for export (x)
	docEntries    []agent.Entry
	docView       transcriptView
	docOpts       transcriptOpts // m, e, t, r and enter
	docSel        int            // fold picked with ]/[ (index in docView.folds), -1 none
	docStack      []docFrame     // parents of an open subagent transcript; esc returns
	docTask       bool           // the open transcript is a subagent's: its prompt is a task
	agentFilter   string         // "" = all agents
	grouped       bool           // by agent+project; not persisted
	selected      map[string]bool
	confirm       *components.Confirm
	deleteTargets []agent.Session
	dirInput      textinput.Model
	pendingResume agent.Session
	aliasInput    textinput.Model
	aliasTarget   agent.Session

	// full-text search: searchIDs != nil filters the list to the hits; esc
	// restores it.
	searchInput   textinput.Model
	searchIDs     map[string]bool
	searchQuery   string
	toast         string
	toastErr      bool
	toastSeq      int // toast auto-dismiss
	spin          spinner.Model
	inFlight      bool
	width, height int

	// per-session token usage: loaded lazily on focus, never during the scan,
	// cached by ID.
	usageCache map[string]agent.Usage
	usageOK    map[string]bool    // sessionID → usage found; a present key means tried
	costCache  map[string]float64 // sessionID → cost, only for API-key accounts
	usageBusy  map[string]bool    // in flight
}

// sessToastExpire clears the toast if it is still number seq.
type sessToastExpire struct{ seq int }

func sessExpireToastCmd(seq int) tea.Cmd {
	return tea.Tick(kit.ToastTTL, func(time.Time) tea.Msg { return sessToastExpire{seq} })
}

func newTab(svc *Service, home string) Tab {
	l := list.New(nil, kit.TableDelegate{}, 0, 0)
	kit.StyleList(&l)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false) // the count is in the title
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(theme.Primary)))
	return Tab{svc: svc, home: home, list: l, vp: viewport.New(), detailVP: viewport.New(), spin: sp}
}

// beginSpin starts the spinner with a label and returns the first tick.
func (m *Tab) beginSpin(label string) tea.Cmd {
	m.toast, m.toastErr = label, false
	if m.inFlight {
		return nil
	}
	m.inFlight = true
	return m.spin.Tick
}

func (m Tab) Init() tea.Cmd { return nil }

func (m *Tab) ClearToast() { m.toast = "" }

func (m Tab) Count() int { return len(m.sessions) }

func (m Tab) Capturing() bool {
	return m.mode != sessModeList || m.list.SettingFilter() || m.confirm != nil
}

// step wraps update() to schedule the toast auto-dismiss.
func (m Tab) step(msg tea.Msg) (Tab, tea.Cmd) {
	prev := m.toast
	var cmd tea.Cmd
	m, cmd = m.update(msg)
	if m.toast != "" && m.toast != prev {
		m.toastSeq++
		cmd = tea.Batch(cmd, sessExpireToastCmd(m.toastSeq))
	}
	return m, cmd
}

func (m Tab) update(msg tea.Msg) (Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.layout()

	case events.AgentsDetected:
		return m, m.loadCmd()

	case sessToastExpire:
		if msg.seq == m.toastSeq && !m.inFlight {
			m.toast = ""
		}
		return m, nil

	case spinner.TickMsg:
		if !m.inFlight {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case events.SessionsLoaded:
		m.inFlight = false
		if msg.Err != nil {
			m.toast, m.toastErr = msg.Err.Error(), true
		}
		m.sessions = msg.Sessions
		return m, m.applyItems()

	case usageMsg:
		delete(m.usageBusy, msg.id)
		if m.usageOK == nil {
			m.usageOK = make(map[string]bool)
			m.usageCache = make(map[string]agent.Usage)
			m.costCache = make(map[string]float64)
		}
		m.usageOK[msg.id] = msg.ok
		if msg.ok {
			m.usageCache[msg.id] = msg.usage
		}
		if msg.hasCost {
			m.costCache[msg.id] = msg.cost
		}
		if it, sel := m.list.SelectedItem().(sessionItem); sel && it.s.ID == msg.id {
			m.refreshDetail()
		}
		return m, nil

	case searchDoneMsg:
		m.inFlight = false
		if len(msg.matches) == 0 {
			if msg.err != nil {
				m.toast, m.toastErr = msg.err.Error(), true
			} else {
				m.toast, m.toastErr = fmt.Sprintf("no session contains %q", msg.query), true
			}
			return m, nil
		}
		ids := make(map[string]bool, len(msg.matches))
		for _, mt := range msg.matches {
			ids[mt.Session.ID] = true
		}
		m.searchIDs, m.searchQuery = ids, msg.query
		if msg.err != nil {
			m.toast, m.toastErr = fmt.Sprintf("%d session(s) contain %q (some could not be read)", len(msg.matches), msg.query), false
		} else {
			m.toast, m.toastErr = fmt.Sprintf("%d session(s) contain %q", len(msg.matches), msg.query), false
		}
		m.list.Select(0)
		return m, m.applyItems()

	case aliasDoneMsg:
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
			return m, nil
		}
		for i := range m.sessions { // apply without reloading every agent
			if m.sessions[i].AgentID+":"+m.sessions[i].ID == msg.key {
				m.sessions[i].Alias = msg.alias
			}
		}
		if msg.alias == "" {
			m.toast, m.toastErr = "alias removed", false
		} else {
			m.toast, m.toastErr = fmt.Sprintf("alias: %s", msg.alias), false
		}
		return m, m.applyItems()

	case resumeDoneMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = fmt.Sprintf("resume failed: %v", msg.err), true
		} else {
			m.toast, m.toastErr = "back in lazyagents", false
		}
		return m, m.loadCmd()

	case transcriptMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
			return m, nil
		}
		m.toast = "" // it was the loading spinner label
		if msg.sub {
			m.docStack = append(m.docStack, docFrame{m.docTitle, m.docSession, m.docEntries, m.docOpts, m.docTask, m.docSel, m.vp.YOffset()})
		} else {
			m.docStack = nil
		}
		m.docTask = msg.task
		m.docTitle = msg.title
		m.docSession = msg.session
		m.docEntries = msg.entries
		m.docOpts, m.docSel = transcriptOpts{}, -1
		m.renderDoc()
		m.vp.GotoTop()
		m.mode = sessModeDoc
		return m, nil

	case exportDoneMsg:
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
		} else {
			m.toast, m.toastErr = fmt.Sprintf("transcript exported to %s", core.Tilde(msg.path, m.home)), false
		}
		return m, nil

	case deleteSessionsMsg:
		m.confirm = nil
		m.selected = nil
		if msg.failed > 0 {
			parts := make([]string, len(msg.errs))
			for i, e := range msg.errs {
				parts[i] = e.Error()
			}
			m.toast, m.toastErr = fmt.Sprintf(
				"%d deleted, %d failed: %s",
				msg.deleted, msg.failed, strings.Join(parts, "; "),
			), true
		} else {
			m.toast, m.toastErr = fmt.Sprintf(
				"%d session(s) moved to backup", msg.deleted,
			), false
		}
		return m, m.loadCmd()

	case tea.MouseWheelMsg:
		if m.confirm != nil {
			c, _ := m.confirm.Update(msg, m.width, m.height)
			m.confirm = &c
			return m, nil
		}
		if m.mode == sessModeDoc {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		if m.mode != sessModeList {
			return m, nil
		}
		if sp := m.split(); sp.Side && msg.X >= sp.ListW || !sp.Side && msg.Y >= sp.ListH {
			var cmd tea.Cmd // wheel over the detail scrolls it
			m.detailVP, cmd = m.detailVP.Update(msg)
			return m, cmd
		}
		if msg.Button == tea.MouseWheelUp {
			m.list.CursorUp()
		} else if msg.Button == tea.MouseWheelDown {
			m.list.CursorDown()
		}
		return m, m.refreshDetail()

	case tea.MouseClickMsg:
		if m.confirm != nil {
			return m, nil
		}
		if m.mode != sessModeList {
			return m, nil // clicks do not close the reader; esc does
		}
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		sp := m.split()
		if sp.Side && msg.X >= sp.ListW || !sp.Side && msg.Y >= sp.ListH {
			return m, nil // only the wheel acts on the detail
		}
		start, end, top := m.tableWindow(sp.ListW, sp.ListH)
		idx := start + msg.Y - top
		if msg.Y < top || idx >= end {
			return m, nil
		}
		if idx == m.list.Index() {
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				svc, s := m.svc, it.s
				spin := m.beginSpin("loading transcript…")
				return m, tea.Batch(spin, func() tea.Msg {
					entries, err := svc.Transcript(s)
					return transcriptMsg{session: s, title: s.Title, entries: entries, err: err}
				})
			}
			return m, nil
		}
		m.list.Select(idx)
		return m, m.refreshDetail()

	case tea.PasteMsg:
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = kit.FeedTextToList(m.list, msg.Content)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.mode == sessModeDoc {
			switch msg.String() {
			case "esc", "q", "v":
				if n := len(m.docStack); n > 0 {
					f := m.docStack[n-1]
					m.docStack = m.docStack[:n-1]
					m.docTitle, m.docSession, m.docEntries, m.docOpts, m.docTask, m.docSel = f.title, f.session, f.entries, f.opts, f.task, f.sel
					m.renderDoc()
					m.vp.SetYOffset(f.y)
					return m, nil
				}
				m.mode = sessModeList
				return m, nil
			case "x":
				svc, sess, entries := m.svc, m.docSession, m.docEntries
				return m, func() tea.Msg {
					path, err := svc.ExportTranscript(sess, entries)
					return exportDoneMsg{path: path, err: err}
				}
			case "n":
				m.jumpPrompt(1)
				return m, nil
			case "N", "p":
				m.jumpPrompt(-1)
				return m, nil
			case "g", "home":
				m.vp.GotoTop()
				return m, nil
			case "G", "end":
				m.vp.GotoBottom()
				return m, nil
			case "r":
				m.toggleDoc(func() { m.docOpts.thinking = !m.docOpts.thinking })
				return m, nil
			case "t":
				m.toggleDoc(func() { m.docOpts.tools = !m.docOpts.tools })
				return m, nil
			case "e":
				// Folding back also drops t, r and the turns opened one by one.
				m.toggleDoc(func() { m.docOpts = transcriptOpts{mode: m.docOpts.mode, steps: m.docOpts.folded()} })
				return m, nil
			case "m":
				m.toggleDoc(func() { m.docOpts = transcriptOpts{mode: (m.docOpts.mode + 1) % len(modeNames)} })
				return m, nil
			case "]":
				m.pickFold(1)
				return m, nil
			case "[":
				m.pickFold(-1)
				return m, nil
			case "enter":
				return m, m.openFold()
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		if m.mode == sessModeSearch {
			return m.updateSearch(msg)
		}
		if m.mode == sessModeDir {
			return m.updateDirPicker(msg)
		}
		if m.mode == sessModeAlias {
			return m.updateAlias(msg)
		}
		if m.confirm != nil {
			c, res := m.confirm.Update(msg, m.width, m.height)
			m.confirm = &c
			switch res {
			case components.Yes:
				targets := m.deleteTargets
				m.confirm, m.deleteTargets = nil, nil
				return m, m.deleteCmd(targets)
			case components.No:
				m.confirm, m.deleteTargets = nil, nil
			}
			return m, nil
		}
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		if kit.DetailScroll[msg.String()] {
			var cmd tea.Cmd
			m.detailVP, cmd = m.detailVP.Update(kit.DetailScrollMsg(msg))
			return m, cmd
		}
		switch msg.String() {
		case "enter":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				return m.resume(it.s)
			}
		case "v":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				svc, s := m.svc, it.s
				spin := m.beginSpin("loading transcript…")
				return m, tea.Batch(spin, func() tea.Msg {
					entries, err := svc.Transcript(s)
					return transcriptMsg{session: s, title: s.Title, entries: entries, err: err}
				})
			}
		case "R":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				_, dir, ok2 := m.svc.ResumeCmd(it.s)
				if !ok2 {
					m.toast, m.toastErr = fmt.Sprintf("%s does not support resume via CLI", it.s.AgentName), true
					return m, nil
				}
				res := m.openDirPicker(it.s, dir)
				return res, nil
			}
			return m, nil
		case "c":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				argv, dir, ok := m.svc.ResumeCmd(it.s)
				if !ok {
					m.toast, m.toastErr = fmt.Sprintf("%s does not support resume via CLI", it.s.AgentName), true
				} else {
					m.toast, m.toastErr = fmt.Sprintf("cd %s && %s", dir, strings.Join(argv, " ")), false
				}
			}
			return m, nil
		case "space":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				cmd := m.toggleSelect(it.s.ID)
				return m, cmd
			}
			if h, ok := m.list.SelectedItem().(sessionGroupHeader); ok {
				return m, m.toggleSelectGroup(h.ids)
			}
		case "g":
			m.grouped = !m.grouped
			m.list.Select(0)
			return m, m.applyItems()
		case "d":
			sel := m.selectedSessions()
			if len(sel) == 0 {
				if it, ok := m.list.SelectedItem().(sessionItem); ok {
					cmd := m.toggleSelect(it.s.ID)
					m.askDelete()
					return m, cmd
				}
				return m, nil
			}
			m.askDelete()
			return m, nil
		case "r":
			return m, tea.Batch(m.beginSpin("reloading…"), m.loadCmd())
		case "f":
			m.agentFilter = m.nextAgentFilter()
			m.list.Select(0)
			return m, m.applyItems()
		case "m":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				return m.openAlias(it.s), nil
			}
			return m, nil
		case "F":
			inp := components.NewInput()
			inp.Placeholder = "search transcripts…"
			inp.SetWidth(60)
			inp.Focus()
			m.searchInput = inp
			m.mode = sessModeSearch
			return m, nil
		case "esc":
			if m.searchIDs != nil {
				m.searchIDs, m.searchQuery = nil, ""
				m.list.Select(0)
				return m, m.applyItems()
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, tea.Batch(cmd, m.refreshDetail()) // the cursor may have moved
	}
	// bubbles' internal messages (e.g. list.FilterMatchesMsg, the async filter
	// result) must reach the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// bodyHeight excludes hints and toast.
func (m Tab) bodyHeight() int {
	h := m.height - 2
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Tab) layout() tea.Cmd {
	if m.width == 0 {
		return nil
	}
	// The list only holds filter and cursor (the table is drawn separately);
	// one line per session so PgUp/PgDn move a screen.
	sp := m.split()
	m.list.SetSize(sp.ListW, max(1, sp.ListH-2))
	m.vp.SetWidth(m.width - m.railWidth())
	m.vp.SetHeight(max(3, m.height-4)) // header (2), hints and toast
	if m.mode == sessModeDoc {
		m.renderDoc()
	}
	return m.refreshDetail()
}

func (m *Tab) renderDoc() {
	m.docOpts.home = m.home
	m.docOpts.task = m.docTask
	m.docView = renderTranscript(m.docEntries, m.width-2-m.railWidth(), m.docSession, m.docOpts)
	m.docSel = min(m.docSel, len(m.docView.folds)-1)
	m.setDocContent()
}

// setDocContent marks the picked fold: its gutter bar, or an event line's
// leading rule, becomes ▶.
func (m *Tab) setDocContent() {
	content := m.docView.content
	if m.docSel >= 0 {
		lines := strings.Split(content, "\n")
		l := m.docView.folds[m.docSel].line
		if strings.Contains(lines[l], "│") {
			lines[l] = strings.Replace(lines[l], "│", "▶", 1)
		} else {
			lines[l] = strings.Replace(lines[l], "──", "▶─", 1)
		}
		content = strings.Join(lines, "\n")
	}
	m.vp.SetContent(content)
}

// railWidth is the prompt rail's width: only on wide terminals, where the
// chat column leaves the sides empty.
func (m Tab) railWidth() int {
	if m.width < 150 {
		return 0
	}
	return 36
}

// pickFold moves the fold cursor (dir 1 = ]) and scrolls to it. Without a
// visible pick it starts from the screen: the first fold on it, or below.
func (m *Tab) pickFold(dir int) {
	folds := m.docView.folds
	if len(folds) == 0 {
		return
	}
	top, h := m.vp.YOffset(), m.vp.Height()
	sel := m.docSel
	if sel >= 0 && (folds[sel].line < top || folds[sel].line >= top+h) {
		sel = -1
	}
	switch {
	case sel >= 0:
		sel = max(0, min(len(folds)-1, sel+dir))
	case dir > 0:
		sel = len(folds) - 1
		for i, f := range folds {
			if f.line >= top {
				sel = i
				break
			}
		}
	default:
		sel = 0
		for i, f := range folds {
			if f.line < top+h {
				sel = i
			}
		}
	}
	m.docSel = sel
	m.setDocContent()
	if l := folds[sel].line; l < top || l >= top+h {
		m.vp.SetYOffset(max(0, l-2))
	}
}

// openFold unfolds or folds the picked turn, keeping it where it was on
// screen, or opens the picked subagent's transcript.
func (m *Tab) openFold() tea.Cmd {
	if m.docSel < 0 {
		return nil
	}
	f := m.docView.folds[m.docSel]
	if f.sub.Sub != "" {
		parent := m.docSession
		_, desc, _ := strings.Cut(f.sub.Text, " · ")
		sub := agent.Session{AgentID: parent.AgentID, AgentName: parent.AgentName, ID: parent.ID + "-subagent",
			CWD: parent.CWD, Path: f.sub.Sub, Title: desc}
		svc, task := m.svc, f.sub.Kind == agent.ToolAgent
		spin := m.beginSpin("loading transcript…")
		return tea.Batch(spin, func() tea.Msg {
			entries, err := svc.Transcript(sub)
			return transcriptMsg{session: sub, title: "↳ " + desc, entries: entries, err: err, sub: true, task: task}
		})
	}
	if m.docOpts.open == nil {
		m.docOpts.open = map[int]bool{}
	}
	m.docOpts.open[f.turn] = !m.docOpts.open[f.turn]
	row := f.line - m.vp.YOffset()
	m.renderDoc()
	for i, g := range m.docView.folds {
		if g.turn == f.turn {
			m.docSel = i
			m.setDocContent()
			m.vp.SetYOffset(max(0, g.line-row))
		}
	}
	return nil
}

// currentPrompt is the index of the prompt being read: the last one at or
// above the top of the screen.
func (m Tab) currentPrompt() int {
	cur := 0
	for i, p := range m.docView.prompts {
		if p <= m.vp.YOffset() {
			cur = i
		}
	}
	return cur
}

// toggleDoc returns to the prompt being read: turn heights change, so the old
// offset would land elsewhere in the chat.
func (m *Tab) toggleDoc(change func()) {
	anchor := -1
	for i, p := range m.docView.prompts {
		if p <= m.vp.YOffset() {
			anchor = i
		}
	}
	change()
	m.docSel = -1
	m.renderDoc()
	if anchor >= 0 {
		m.vp.SetYOffset(m.docView.prompts[anchor])
	}
}

// jumpPrompt moves to the next (dir=1) or previous (dir=-1) user prompt.
func (m *Tab) jumpPrompt(dir int) {
	y := m.vp.YOffset()
	ps := m.docView.prompts
	if dir > 0 {
		for _, p := range ps {
			if p > y {
				m.vp.SetYOffset(p)
				return
			}
		}
		m.vp.GotoBottom()
		return
	}
	for i := len(ps) - 1; i >= 0; i-- {
		if ps[i] < y {
			m.vp.SetYOffset(ps[i])
			return
		}
	}
	m.vp.GotoTop()
}

func (m *Tab) ID() string { return "sessions" }

func (m *Tab) Title() string { return "Sessions" }

// Update treats events.Reload as the r key.
func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(events.Reload); ok {
		msg = tea.KeyPressMsg{Code: 'r', Text: "r"}
	}
	nm, cmd := m.step(msg)
	*m = nm
	return cmd
}
