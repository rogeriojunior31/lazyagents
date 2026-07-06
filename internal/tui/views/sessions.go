package views

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/session"
)

type sessMode int

const (
	sessModeList sessMode = iota
	sessModeDoc           // lendo o transcript de uma sessão
)

// Sessions é a aba de sessões unificadas de todos os agentes. enter suspende a
// TUI e retoma a sessão no CLI de origem; v abre o transcript para leitura.
type Sessions struct {
	svc           *session.Service
	home          string
	sessions      []agent.Session
	list          list.Model
	vp            viewport.Model
	mode          sessMode
	docTitle      string
	agentFilter   string // "" = todas; senão, só sessões desse agente
	toast         string
	toastErr      bool
	width, height int
}

type sessionsMsg struct {
	sessions []agent.Session
	err      error
}

type resumeDoneMsg struct{ err error }

type transcriptMsg struct {
	title   string
	entries []agent.Entry
	err     error
}

type sessionItem struct {
	s     agent.Session
	title string
	desc  string
}

func (i sessionItem) Title() string       { return i.title }
func (i sessionItem) Description() string { return i.desc }

// FilterValue: só agente + título. O cwd fica de fora de propósito — o fuzzy
// casando letras espalhadas pelos paths tornava o filtro inútil.
func (i sessionItem) FilterValue() string {
	return tagLabel(i.s.AgentID) + " " + i.s.Title
}

var tagStyles = map[string]lipgloss.Style{
	"claude-code": lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")),
	"codex":       lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")),
	"gemini-cli":  lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")),
	"opencode":    lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")),
}

func tagLabel(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, "-cli"), "-code")
}

// agentTag devolve a tag colorida do agente, com largura fixa para os títulos
// da lista ficarem alinhados em coluna.
func agentTag(id string) string {
	label := tagLabel(id)
	pad := strings.Repeat(" ", max(0, 8-len(label)))
	st, ok := tagStyles[id]
	if !ok {
		st = stHint
	}
	return st.Render("⏺ "+label) + pad
}

// relTime formata a idade da sessão de forma humana.
func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("há %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("há %dh", int(d.Hours()))
	case d < 48*time.Hour:
		return "ontem"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("há %dd", int(d.Hours()/24))
	default:
		return t.Format("02/01/2006")
	}
}

func newSessionItem(s agent.Session, home string) sessionItem {
	cwd := s.CWD
	if cwd == "" {
		cwd = "(pasta desconhecida)"
	}
	return sessionItem{
		s:     s,
		title: agentTag(s.AgentID) + " " + s.Title,
		desc:  "  " + relTime(s.MTime) + " · " + tilde(cwd, home),
	}
}

func NewSessions(svc *session.Service, home string) Sessions {
	l := list.New(nil, plainDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return Sessions{svc: svc, home: home, list: l, vp: viewport.New()}
}

func (m Sessions) Init() tea.Cmd { return nil }

func (m Sessions) Capturing() bool {
	return m.mode != sessModeList || m.list.SettingFilter()
}

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
		return m, m.applyItems()

	case resumeDoneMsg:
		if msg.err != nil {
			m.toast, m.toastErr = "resume terminou com erro: "+msg.err.Error(), true
		} else {
			m.toast, m.toastErr = "de volta ao lazyskills", false
		}
		return m, m.loadCmd()

	case transcriptMsg:
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
			return m, nil
		}
		m.docTitle = msg.title
		m.vp.SetContent(renderTranscript(msg.entries, m.width-2))
		m.vp.GotoTop()
		m.mode = sessModeDoc
		return m, nil

	case tea.MouseWheelMsg:
		if m.mode == sessModeDoc {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		if msg.Button == tea.MouseWheelUp {
			m.list.CursorUp()
		} else if msg.Button == tea.MouseWheelDown {
			m.list.CursorDown()
		}
		return m, nil

	case tea.MouseClickMsg:
		if m.mode == sessModeDoc {
			m.mode = sessModeList // clique fecha a leitura
			return m, nil
		}
		if msg.Button != tea.MouseLeft || msg.X >= m.listWidth() {
			return m, nil
		}
		idx := listIndexAt(&m.list, msg.Y)
		if idx < 0 {
			return m, nil
		}
		if idx == m.list.Index() {
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				return m.resume(it.s) // segundo clique retoma a sessão
			}
			return m, nil
		}
		m.list.Select(idx)
		return m, nil

	case tea.PasteMsg:
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = feedTextToList(m.list, msg.Content)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.mode == sessModeDoc {
			switch msg.String() {
			case "esc", "q", "v":
				m.mode = sessModeList
				return m, nil
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
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
		case "v":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				svc, s := m.svc, it.s
				m.toast, m.toastErr = "carregando transcript…", false
				return m, func() tea.Msg {
					entries, err := svc.Transcript(s)
					return transcriptMsg{title: s.Title, entries: entries, err: err}
				}
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
		case "f":
			m.agentFilter = m.nextAgentFilter()
			m.list.Select(0)
			return m, m.applyItems()
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	// mensagens internas dos bubbles (ex.: list.FilterMatchesMsg, que entrega
	// o resultado assíncrono do filtro) precisam chegar à lista
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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

// applyItems repõe os itens da lista respeitando o filtro de agente ativo.
func (m *Sessions) applyItems() tea.Cmd {
	items := make([]list.Item, 0, len(m.sessions))
	for _, s := range m.sessions {
		if m.agentFilter != "" && s.AgentID != m.agentFilter {
			continue
		}
		items = append(items, newSessionItem(s, m.home))
	}
	return m.list.SetItems(items)
}

// nextAgentFilter cicla todas → cada agente com sessões → todas.
func (m Sessions) nextAgentFilter() string {
	var cycle []string
	seen := map[string]bool{}
	for _, s := range m.sessions {
		if !seen[s.AgentID] {
			seen[s.AgentID] = true
			cycle = append(cycle, s.AgentID)
		}
	}
	if len(cycle) == 0 {
		return ""
	}
	for i, id := range cycle {
		if id == m.agentFilter {
			if i+1 < len(cycle) {
				return cycle[i+1]
			}
			return "" // fim do ciclo: volta para todas
		}
	}
	return cycle[0]
}

func (m Sessions) listWidth() int { return m.width * 3 / 5 }

func (m *Sessions) layout() {
	if m.width == 0 {
		return
	}
	bodyH := m.height - 2
	if bodyH < 3 {
		bodyH = 3
	}
	m.list.SetSize(m.listWidth(), bodyH)
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(bodyH)
}

// renderTranscript formata as mensagens para o viewport de leitura.
func renderTranscript(entries []agent.Entry, width int) string {
	if len(entries) == 0 {
		return stHint.Render("(transcript vazio ou em formato desconhecido)")
	}
	var b strings.Builder
	for _, e := range entries {
		if e.Role == "user" {
			b.WriteString(stOn.Render("▶ você") + "\n")
		} else {
			b.WriteString(stShared.Render("◀ agente") + "\n")
		}
		b.WriteString(renderMarkdown(e.Text, width) + "\n")
	}
	return b.String()
}

// detailView é o card lateral com os dados da sessão selecionada.
func (m Sessions) detailView(w int) string {
	it, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return cardOff.Width(w).Render(stHint.Render("Nenhuma sessão encontrada."))
	}
	s := it.s
	inner := w - 4
	label := func(l string) string { return cardLabel.Render(fmt.Sprintf("%-8s", l)) }
	var b strings.Builder
	b.WriteString(stTitle.Render(truncate(s.Title, 200)) + "\n\n")
	st, okTag := tagStyles[s.AgentID]
	if !okTag {
		st = stHint
	}
	b.WriteString(label("agente") + st.Render(s.AgentName) + "\n")
	b.WriteString(label("quando") + cardValue.Render(relTime(s.MTime)) +
		cardLabel.Render("  ("+s.MTime.Format("02/01/2006 15:04")+")") + "\n")
	if s.CWD != "" {
		b.WriteString(label("pasta") + cardValue.Render(tilde(s.CWD, m.home)) + "\n")
	}
	b.WriteString(label("id") + cardLabel.Render(s.ID) + "\n")
	if argv, dir, okCmd := m.svc.ResumeCmd(s); okCmd {
		b.WriteString("\n" + cardLabel.Render("retomar  ") + "\n" +
			mdCode.Render(truncate("cd "+tilde(dir, m.home)+" && "+strings.Join(argv, " "), 3*inner)))
	}
	return cardOn.Width(w).Render(lipgloss.NewStyle().Width(inner).Render(b.String()))
}

func (m Sessions) View() string {
	if m.mode == sessModeDoc {
		head := stTitle.Render(truncate(m.docTitle, 100)) +
			stHint.Render("  transcript · esc volta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	}
	detailW := m.width - m.listWidth() - 3
	if detailW < 24 {
		detailW = 24
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), "  ", m.detailView(detailW))
	filterHint := stHint.Render("f agente")
	if m.agentFilter != "" {
		st, ok := tagStyles[m.agentFilter]
		if !ok {
			st = stHint
		}
		filterHint = stText.Render("f agente: ") + st.Render("⏺ "+tagLabel(m.agentFilter))
	}
	hints := stHint.Render("enter retoma · v lê o transcript · c comando · / filtra · ") +
		filterHint + stHint.Render(" · r recarrega")
	toast := ""
	if m.toast != "" {
		if m.toastErr {
			toast = stErr.Render("✗ " + m.toast)
		} else {
			toast = stOn.Render("✓ " + m.toast)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, toast)
}
