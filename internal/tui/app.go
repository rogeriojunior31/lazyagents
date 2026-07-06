// Package tui é a interface Bubble Tea do lazyskills: abas Skills, Sessões e
// Agentes. Todo I/O acontece nos services, dentro de tea.Cmd.
package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/session"
	"lazyskills/internal/skill"
	"lazyskills/internal/tui/views"
)

type tab int

const (
	tabSkills tab = iota
	tabSessions
	tabAgents
	tabCount
)

var tabNames = []string{"Skills", "Sessões", "Agentes"}

// Model é o root: roteia teclas para a aba ativa e faz broadcast de mensagens
// assíncronas para todas as views.
type Model struct {
	keys     keyMap
	styles   styles
	help     help.Model
	adapters []agent.Adapter

	active   tab
	width    int
	height   int
	skills   views.Skills
	sessions views.Sessions
	agents   views.Agents
}

func New(adapters []agent.Adapter, skillSvc *skill.Service, sessionSvc *session.Service) Model {
	return Model{
		keys:     newKeyMap(),
		styles:   newStyles(),
		help:     help.New(),
		adapters: adapters,
		skills:   views.NewSkills(skillSvc),
		sessions: views.NewSessions(sessionSvc),
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

func (m Model) Init() tea.Cmd { return m.detectCmd() }

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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		inner := tea.WindowSizeMsg{Width: msg.Width - 4, Height: msg.Height - 6}
		return m.updateViews(inner)

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
	v.WindowTitle = "lazyskills"
	if m.width == 0 {
		v.Content = "carregando…"
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
