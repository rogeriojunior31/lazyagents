// Package tui é a interface Bubble Tea do lazyskills: abas Skills, Sessões e
// Agentes. Todo I/O acontece nos services, dentro de tea.Cmd.
package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/session"
	"lazyskills/internal/skill"
	"lazyskills/internal/tui/views"
)

type appState int

const (
	stateSplash appState = iota
	stateMain
)

type splashDoneMsg struct{}

func splashTimerCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)
		return splashDoneMsg{}
	}
}

type tab int

const (
	tabSkills tab = iota
	tabSessions
	tabAgents
	tabCount
)

var tabNames = []string{"Skills", "Sessões", "Agentes"}

// Offsets do layout do View(), usados para traduzir cliques do mouse:
// linha 0 título, linha 1 abas, linha 2 separador, corpo com Padding(1,2).
const (
	tabRowY     = 1
	bodyOriginY = 4 // sep (y=2) + padding-top do body
	bodyOriginX = 2 // padding-left do body
)

// Model é o root: roteia teclas para a aba ativa e faz broadcast de mensagens
// assíncronas para todas as views.
type Model struct {
	keys     keyMap
	styles   styles
	help     help.Model
	adapters []agent.Adapter

	state    appState
	splash   views.Splash
	active   tab
	width    int
	height   int
	skills   views.Skills
	sessions views.Sessions
	agents   views.Agents
}

func New(adapters []agent.Adapter, skillSvc *skill.Service, sessionSvc *session.Service, version string) Model {
	return Model{
		keys:     newKeyMap(),
		styles:   newStyles(),
		help:     help.New(),
		adapters: adapters,
		state:    stateSplash,
		splash:   views.NewSplash(version),
		skills:   views.NewSkills(skillSvc),
		sessions: views.NewSessions(sessionSvc, skillSvc.Paths().Home),
		agents:   views.NewAgents(),
	}
}

// detectCmd roda a detecção dos agentes (inclui --version de cada CLI) fora do
// Update; o resultado alimenta todas as views.
func (m Model) detectCmd() tea.Cmd {
	adapters := m.adapters
	return func() tea.Msg {
		return views.AgentsMsg{Agents: agent.DetectAll(adapters)}
	}
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.detectCmd(), splashTimerCmd()) }

func (m Model) capturingInput() bool {
	switch m.active {
	case tabSkills:
		return m.skills.Capturing()
	case tabSessions:
		return m.sessions.Capturing()
	default:
		return false
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// splash: só Enter/espaço/timer avançam; mensagens async passam para as views
	// para que skills e sessões carreguem em background durante o splash.
	if m.state == stateSplash {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width, m.height = msg.Width, msg.Height
			m.splash = m.splash.Resize(msg.Width, msg.Height-6)
			inner := tea.WindowSizeMsg{Width: msg.Width - 4, Height: msg.Height - 6}
			return m.updateViews(inner)
		case splashDoneMsg:
			m.state = stateMain
			return m, nil
		case tea.KeyPressMsg:
			if msg.String() == "enter" || msg.String() == "space" {
				m.state = stateMain
			}
			return m, nil
		default:
			// AgentsMsg, skillsScanMsg, sessionsMsg etc. — deixar as views processar
			// em background enquanto o splash está visível.
			return m.updateViews(msg)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		inner := tea.WindowSizeMsg{Width: msg.Width - 4, Height: msg.Height - 6}
		return m.updateViews(inner)

	case splashDoneMsg:
		return m, nil // já estamos no stateMain; ignorar timer tardio

	case tea.KeyPressMsg:
		if !m.capturingInput() {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Help):
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			case key.Matches(msg, m.keys.NextTab):
				m.active = (m.active + 1) % tabCount
				return m, nil
			case key.Matches(msg, m.keys.PrevTab):
				m.active = (m.active + tabCount - 1) % tabCount
				return m, nil
			}
		}
		return m.updateActive(msg)

	case tea.MouseWheelMsg:
		return m.updateActive(msg)

	case tea.PasteMsg:
		// texto colado vai só para a aba ativa (input de install ou filtro)
		return m.updateActive(msg)

	case tea.MouseClickMsg:
		// clique na linha das abas troca de aba
		if msg.Y == tabRowY {
			x := 0
			for i, name := range tabNames {
				w := lipgloss.Width(m.styles.tab.Render(name))
				if msg.X >= x && msg.X < x+w {
					m.active = tab(i)
					return m, nil
				}
				x += w
			}
			return m, nil
		}
		// demais cliques: traduz para coordenadas do corpo e delega à view
		translated := tea.MouseClickMsg(msg.Mouse())
		translated.X -= bodyOriginX
		translated.Y -= bodyOriginY
		if translated.Y < 0 {
			return m, nil
		}
		return m.updateActive(translated)

	default:
		// mensagens assíncronas (scans, sessões, resultados de ops) vão para
		// todas as views — Agentes conta sessões, Skills refaz a matriz etc.
		return m.updateViews(msg)
	}
}

func (m Model) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.active {
	case tabSkills:
		m.skills, cmd = m.skills.Update(msg)
	case tabSessions:
		m.sessions, cmd = m.sessions.Update(msg)
	case tabAgents:
		m.agents, cmd = m.agents.Update(msg)
	}
	return m, cmd
}

func (m Model) updateViews(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.skills, cmd = m.skills.Update(msg)
	cmds = append(cmds, cmd)
	m.sessions, cmd = m.sessions.Update(msg)
	cmds = append(cmds, cmd)
	m.agents, cmd = m.agents.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "lazyskills"
	if m.width == 0 {
		v.Content = "carregando…"
		return v
	}
	if m.state == stateSplash {
		v.Content = m.splash.View()
		return v
	}

	var tabs []string
	for i, name := range tabNames {
		if tab(i) == m.active {
			tabs = append(tabs, m.styles.activeTab.Render(name))
		} else {
			tabs = append(tabs, m.styles.tab.Render(name))
		}
	}
	title := m.styles.title.Render("lazyskills") +
		m.styles.tagline.Render("skills e sessões de todos os seus agentes")
	sep := m.styles.separator.Render(strings.Repeat("─", max(0, m.width)))

	var body string
	switch m.active {
	case tabSkills:
		body = m.skills.View()
	case tabSessions:
		body = m.sessions.View()
	case tabAgents:
		body = m.agents.View()
	}

	v.Content = lipgloss.JoinVertical(lipgloss.Left,
		title,
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
		sep,
		m.styles.body.Render(body),
		sep,
		m.help.View(m.keys),
	)
	return v
}
