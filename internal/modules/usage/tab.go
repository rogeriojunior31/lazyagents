package usage

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// Tab is the usage tab: how much of the subscription each window (session,
// week) has used, and transcript tokens filtered by period, agent, view and
// text. Nothing loads at boot, only when the tab opens. Pointer semantics
// (module.Module).
type Tab struct {
	svc      *Service
	statuses []Status
	api      map[string]bool // agents authenticated with an API key: the cost column
	events   []agent.UsageEvent // the whole history; filters slice it in memory
	sessions []agent.Session
	names    map[string]string // id → display name (events.AgentsDetected)

	f         filters
	filtering bool // text input (/) open: owns the keyboard
	input     textinput.Model

	// rendered body: rebuilding it aggregates the whole history, so it only
	// happens when what it shows changes (scrolling just slices lines)
	bar   string // pinned filters, memoized with the body
	lines []string
	drawn renderKey
	gen   int // bumped on every limits or events load

	loaded        bool // loaded once already (no network on every tab switch)
	loading       bool
	pending       int // limit and usage queries still running
	scroll        int
	width, height int
	toast         string
	toastErr      bool
}

func newTab(svc *Service, cfg config) Tab { return Tab{svc: svc, f: newFilters(cfg)} }

func (m Tab) Init() tea.Cmd   { return nil } // loads only when the tab opens
func (m *Tab) ID() string     { return "usage" }
func (m *Tab) Title() string  { return "Usage" }
func (m Tab) Count() int      { return -1 }
func (m Tab) Capturing() bool { return m.filtering }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd fetches (cached) limits and the events of every session. With the
// transcript index, reading the whole history costs milliseconds after the
// first time, and the period is just an in-memory slice.
func (m *Tab) loadCmd(refresh bool) tea.Cmd {
	svc, sessions := m.svc, m.sessions
	m.loading = true
	m.pending += 2
	status := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return statusMsg{statuses: svc.Status(ctx, refresh), api: svc.apiKeyAgents()}
	}
	agg := func() tea.Msg { return eventsMsg{events: svc.Events(sessions)} }
	return tea.Batch(status, agg)
}

// renderKey is everything the body depends on; the minute keeps countdowns
// (reset, current block) current.
type renderKey struct {
	f       filters
	width   int
	gen     int
	minute  int64
	loading bool
	typing  bool // the text filter leaves the bar while the input is open
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	cmd := m.update(msg)
	key := renderKey{m.f, m.width, m.gen, time.Now().Unix() / 60, m.loading, m.filtering}
	if m.lines == nil || key != m.drawn {
		m.lines, m.drawn = m.bodyLines(), key
		m.bar = lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.filterBar())
	}
	m.scroll = min(m.scroll, max(0, len(m.lines)-m.contentHeight())) // never scroll past the end
	return cmd
}

func (m *Tab) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case events.AgentsDetected:
		m.names = map[string]string{}
		for _, a := range msg.Agents {
			m.names[a.ID] = a.Name
		}
		m.gen++ // labels change: redraw

	case events.SessionsLoaded:
		m.sessions = msg.Sessions
		if m.loaded { // visited before: re-aggregate with the new list, no network
			return m.loadCmd(false)
		}

	case events.TabActivated:
		if msg.ID == m.ID() && !m.loaded && !m.loading {
			m.loaded = true
			m.toast = "fetching limits…"
			return m.loadCmd(false)
		}

	case refreshMsg: // palette: usage refresh
		return m.refresh()

	case events.Reload:
		return m.refresh()

	case filterMsg: // palette: usage period/view/agent …
		msg.apply(&m.f)
		m.scroll = 0

	case statusMsg:
		m.gen++
		m.finishLoad()
		m.statuses, m.api = msg.statuses, msg.api
		m.toast, m.toastErr = "", false
		warnings := 0
		for _, st := range msg.statuses {
			if st.Err != "" {
				warnings++
			}
		}
		if warnings > 0 {
			m.toast, m.toastErr = fmt.Sprintf("%d warning(s) · see Limits", warnings), true
		}

	case eventsMsg:
		m.finishLoad()
		m.gen++
		m.events = msg.events

	case tea.PasteMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}

	case tea.KeyPressMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateKeys(msg)

	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			m.scroll = max(0, m.scroll-1)
		} else if msg.Button == tea.MouseWheelDown {
			m.scroll++
		}
	}
	return nil
}

func (m *Tab) refresh() tea.Cmd {
	m.loaded = true
	m.toast, m.toastErr = "refreshing limits…", false
	return m.loadCmd(true)
}

// updateKeys handles keys outside the text input.
func (m *Tab) updateKeys(msg tea.KeyPressMsg) tea.Cmd {
	step := 1
	switch msg.String() {
	case "r":
		return m.refresh()
	case "P":
		step = -1
		fallthrough
	case "p":
		m.f.period = (m.f.period + step + len(periods)) % len(periods)
		m.scroll = 0
	case "A":
		step = -1
		fallthrough
	case "a":
		m.f.agent = cycle(m.f.agent, agentsIn(m.statuses, m.events), step)
		m.scroll = 0
	case "left", "h", "V":
		step = -1
		fallthrough
	case "right", "l", "v":
		m.f.view = (m.f.view + step + len(tabViews)) % len(tabViews)
		m.scroll = 0
	case "/":
		m.input = components.NewInput()
		m.input.Placeholder = fmt.Sprintf("filter by %s…", tabViews[m.f.view].label)
		m.input.SetValue(m.f.text)
		m.input.SetWidth(40)
		m.input.Focus()
		m.filtering = true
	case "esc":
		if m.f.text != "" {
			m.f.text = ""
		} else {
			m.f.agent = ""
		}
		m.scroll = 0
	case "up", "k":
		m.scroll = max(0, m.scroll-1)
	case "down", "j":
		m.scroll++
	case "pgup":
		m.scroll = max(0, m.scroll-max(1, m.contentHeight()))
	case "pgdown", "space":
		m.scroll += max(1, m.contentHeight())
	case "g", "home":
		m.scroll = 0
	}
	return nil
}

// updateFilter is the text input: filters while typing; enter closes and
// keeps it, esc closes and clears it.
func (m *Tab) updateFilter(msg tea.Msg) tea.Cmd {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "esc":
			m.filtering, m.f.text = false, ""
			m.input.Blur()
			return nil
		case "enter":
			m.filtering = false
			m.input.Blur()
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.f.text = m.input.Value()
	m.scroll = 0
	return cmd
}

func (m *Tab) finishLoad() {
	m.pending = max(0, m.pending-1)
	m.loading = m.pending > 0
}
