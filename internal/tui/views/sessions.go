package views

import (
	"fmt"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/session"
)

// Sessions é a aba de sessões unificadas de todos os agentes. enter suspende a
// TUI e retoma a sessão no CLI de origem; ao sair do CLI, a TUI volta.
type Sessions struct {
	svc           *session.Service
	sessions      []agent.Session
	list          list.Model
	toast         string
	toastErr      bool
	width, height int
}

type sessionsMsg struct {
	sessions []agent.Session
	err      error
}

type resumeDoneMsg struct{ err error }

type sessionItem struct{ s agent.Session }

func (i sessionItem) Title() string {
	return agentTag(i.s.AgentID) + " " + i.s.Title
}

func (i sessionItem) Description() string {
	cwd := i.s.CWD
	if cwd == "" {
		cwd = "(cwd desconhecido)"
	}
	return fmt.Sprintf("%s · %s", i.s.MTime.Format("02/01 15:04"), cwd)
}

func (i sessionItem) FilterValue() string {
	return i.s.AgentName + " " + i.s.Title + " " + i.s.CWD
}

var tagStyles = map[string]lipgloss.Style{
	"claude-code": lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")),
	"codex":       lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")),
	"gemini-cli":  lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")),
	"opencode":    lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")),
}

func agentTag(id string) string {
	label := "[" + strings.TrimSuffix(strings.TrimSuffix(id, "-cli"), "-code") + "]"
	if st, ok := tagStyles[id]; ok {
		return st.Render(label)
	}
	return stHint.Render(label)
}

func NewSessions(svc *session.Service) Sessions {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return Sessions{svc: svc, list: l}
}

func (m Sessions) Init() tea.Cmd { return nil }

func (m Sessions) Capturing() bool { return m.list.SettingFilter() }

func (m Sessions) loadCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		sessions, err := svc.List()
		return sessionsMsg{sessions: sessions, err: err}
	}
}

func (m Sessions) Update(msg tea.Msg) (Sessions, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case AgentsMsg:
		return m, m.loadCmd()

	case sessionsMsg:
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
		}
		m.sessions = msg.sessions
		items := make([]list.Item, 0, len(msg.sessions))
		for _, s := range msg.sessions {
			items = append(items, sessionItem{s: s})
		}
		return m, m.list.SetItems(items)

	case resumeDoneMsg:
		if msg.err != nil {
			m.toast, m.toastErr = "resume terminou com erro: "+msg.err.Error(), true
		} else {
			m.toast, m.toastErr = "de volta ao lazyskills", false
		}
		return m, m.loadCmd()

	case tea.KeyPressMsg:
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				return m.resume(it.s)
			}
		case "c":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				argv, dir, ok := m.svc.ResumeCmd(it.s)
				if !ok {
					m.toast, m.toastErr = it.s.AgentName+" não suporta resume via CLI", true
				} else {
					m.toast, m.toastErr = fmt.Sprintf("cd %s && %s", dir, strings.Join(argv, " ")), false
				}
			}
			return m, nil
		case "r":
			m.toast, m.toastErr = "recarregando…", false
			return m, m.loadCmd()
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

// resume suspende a TUI e executa o CLI de origem no diretório da sessão.
func (m Sessions) resume(s agent.Session) (Sessions, tea.Cmd) {
	argv, dir, ok := m.svc.ResumeCmd(s)
	if !ok {
		m.toast, m.toastErr = s.AgentName+" não suporta resume via CLI", true
		return m, nil
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		m.toast, m.toastErr = argv[0]+" não está no PATH", true
		return m, nil
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return resumeDoneMsg{err: err}
	})
}

func (m *Sessions) layout() {
	if m.width == 0 {
		return
	}
	bodyH := m.height - 2
	if bodyH < 3 {
		bodyH = 3
	}
	m.list.SetSize(m.width, bodyH)
}

func (m Sessions) View() string {
	hints := stHint.Render("enter retoma a sessão · c mostra o comando · / filtra · r recarrega")
	toast := ""
	if m.toast != "" {
		if m.toastErr {
			toast = stErr.Render(m.toast)
		} else {
			toast = stOn.Render(m.toast)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), hints, toast)
}
