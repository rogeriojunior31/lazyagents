// Package tui é a interface Bubble Tea do lazyagents: abas Skills, Sessões e
// Agentes. Todo I/O acontece nos services, dentro de tea.Cmd.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/session"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/views"
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

// tabLabel monta o texto da aba com o contador dinâmico da respectiva view.
func (m Model) tabLabel(t tab) string {
	switch t {
	case tabSkills:
		return fmt.Sprintf("Skills %d", m.skills.Count())
	case tabSessions:
		return fmt.Sprintf("Sessões %d", m.sessions.Count())
	case tabAgents:
		return fmt.Sprintf("Agentes %d", m.agents.InstalledCount())
	}
	return ""
}

// renderPill devolve a aba em estilo pill: ● preenchida quando ativa, ○ inativa.
func (m Model) renderPill(t tab) string {
	if t == m.active {
		return m.styles.pillOn.Render("● " + m.tabLabel(t))
	}
	return m.styles.pill.Render("○ " + m.tabLabel(t))
}

// Offsets do layout do View(), usados para traduzir cliques do mouse:
// linha 0 header (badge + status), linha 1 abas pill, corpo com Padding(1,2).
const (
	tabRowY     = 1
	bodyOriginY = 3 // header (0) + abas (1) + padding-top do body
	bodyOriginX = 2 // padding-left do body
)

// Model é o root: roteia teclas para a aba ativa e faz broadcast de mensagens
// assíncronas para todas as views.
type Model struct {
	keys     keyMap
	styles   styles
	help     help.Model
	adapters []agent.Adapter

	version     string
	state       appState
	splash      views.Splash
	showHelp    bool // modal de ajuda (?) aberto sobre a aba ativa
	showPalette bool // paleta de comandos (:) aberta sobre a aba ativa
	palette     components.Palette
	active      tab
	width       int
	height      int
	skills      views.Skills
	sessions    views.Sessions
	agents      views.Agents
}

// paletteCommands é o catálogo fixo da paleta (M8.C1): só ações globais —
// nada específico de uma aba, para não invadir o território das outras lanes.
func paletteCommands() []components.Command {
	return []components.Command{
		{Name: "skills", Desc: "abre a aba Skills"},
		{Name: "sessions", Desc: "abre a aba Sessões"},
		{Name: "agents", Desc: "abre a aba Agentes"},
		{Name: "help", Desc: "abre a ajuda da aba atual"},
		{Name: "reload", Desc: "recarrega a aba atual"},
		{Name: "quit", Desc: "sai do lazyagents"},
	}
}

func New(adapters []agent.Adapter, skillSvc *skill.Service, sessionSvc *session.Service, version string) Model {
	return Model{
		keys:     newKeyMap(),
		styles:   newStyles(),
		help:     newHelp(),
		adapters: adapters,
		version:  version,
		state:    stateSplash,
		splash:   views.NewSplash(version),
		palette:  components.NewPalette(paletteCommands()),
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

// newHelp estiliza o rodapé de ajuda global com teclas em keycaps.
func newHelp() help.Model {
	h := help.New()
	chip := components.KeycapStyle
	desc := lipgloss.NewStyle().Foreground(colorSubtle)
	sep := lipgloss.NewStyle().Foreground(colorBorder)
	h.Styles.ShortKey, h.Styles.FullKey = chip, chip
	h.Styles.ShortDesc, h.Styles.FullDesc = desc, desc
	h.Styles.ShortSeparator, h.Styles.FullSeparator = sep, sep
	return h
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
		// paleta de comandos aberta: teclado vai inteiro para ela.
		if m.showPalette {
			var done bool
			var choice string
			var cmd tea.Cmd
			m.palette, cmd, done, choice = m.palette.Update(msg)
			if done {
				m.showPalette = false
				if choice != "" {
					return m.runPaletteCommand(choice)
				}
			}
			return m, cmd
		}
		// modal de ajuda aberto: esc/q/?/enter/space fecham; resto é ignorado.
		if m.showHelp {
			switch msg.String() {
			case "esc", "q", "?", "enter", "space":
				m.showHelp = false
			}
			return m, nil
		}
		if !m.capturingInput() {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Help):
				m.showHelp = true
				return m, nil
			case key.Matches(msg, m.keys.Palette):
				var cmd tea.Cmd
				m.palette, cmd = m.palette.Open()
				m.showPalette = true
				return m, cmd
			case key.Matches(msg, m.keys.NextTab):
				m.active = (m.active + 1) % tabCount
				m.clearToasts()
				return m, nil
			case key.Matches(msg, m.keys.PrevTab):
				m.active = (m.active + tabCount - 1) % tabCount
				m.clearToasts()
				return m, nil
			}
		}
		return m.updateActive(msg)

	case tea.MouseWheelMsg:
		if m.showHelp || m.showPalette {
			return m, nil
		}
		return m.updateActive(msg)

	case tea.PasteMsg:
		if m.showPalette {
			return m, nil
		}
		// texto colado vai só para a aba ativa (input de install ou filtro)
		return m.updateActive(msg)

	case tea.MouseClickMsg:
		if m.showPalette {
			m.showPalette = false
			return m, nil
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		// clique na linha das abas troca de aba
		if msg.Y == tabRowY {
			x := 0
			for i := 0; i < int(tabCount); i++ {
				w := lipgloss.Width(m.renderPill(tab(i)))
				if msg.X >= x && msg.X < x+w {
					m.active = tab(i)
					m.clearToasts()
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

// clearToasts some com os toasts das views ao trocar de aba (M7.3).
func (m *Model) clearToasts() {
	m.skills.ClearToast()
	m.sessions.ClearToast()
}

// runPaletteCommand executa o comando escolhido na paleta (M8.C1). Comandos
// globais só: trocar de aba, abrir ajuda, sair ou recarregar a aba ativa —
// "recarregar" sintetiza a tecla `r` já tratada por cada view, sem invadir o
// território das outras lanes.
func (m Model) runPaletteCommand(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "skills":
		m.active = tabSkills
		m.clearToasts()
		return m, nil
	case "sessions":
		m.active = tabSessions
		m.clearToasts()
		return m, nil
	case "agents":
		m.active = tabAgents
		m.clearToasts()
		return m, nil
	case "help":
		m.showHelp = true
		return m, nil
	case "quit":
		return m, tea.Quit
	case "reload":
		return m.updateActive(tea.KeyPressMsg{Code: 'r', Text: "r"})
	}
	return m, nil
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
	v.WindowTitle = "lazyagents"
	if m.width == 0 {
		v.Content = "carregando…"
		return v
	}
	if m.state == stateSplash {
		v.Content = m.splash.View()
		return v
	}

	var tabs []string
	for i := 0; i < int(tabCount); i++ {
		tabs = append(tabs, m.renderPill(tab(i)))
	}

	// Header: badge + tagline à esquerda, contadores à direita.
	left := m.styles.badge.Render("lazyagents") +
		m.styles.tagline.Render("skills e sessões dos seus agentes")
	status := m.styles.status.Render(fmt.Sprintf("%d skills · %d sessões · v%s",
		m.skills.Count(), m.sessions.Count(), m.version))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(status)
	if gap < 1 {
		gap = 1
	}
	header := left + strings.Repeat(" ", gap) + status

	var body string
	switch {
	case m.showPalette:
		body = m.renderPalette()
	case m.showHelp:
		body = m.renderHelp()
	case m.active == tabSkills:
		body = m.skills.View()
	case m.active == tabSessions:
		body = m.sessions.View()
	case m.active == tabAgents:
		body = m.agents.View()
	}

	v.Content = lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
		m.styles.body.Render(body),
		m.help.View(m.keys),
	)
	return v
}

// renderPalette dimensiona a paleta de comandos ao mesmo padrão do modal de
// ajuda (largura máxima confortável, encolhe em terminal estreito).
func (m Model) renderPalette() string {
	w := m.width - 4
	if w > 50 {
		w = 50
	}
	return m.palette.View(w)
}

// activeHelp devolve os grupos de teclas da aba ativa.
func (m Model) activeHelp() []views.HelpGroup {
	switch m.active {
	case tabSkills:
		return m.skills.Help()
	case tabSessions:
		return m.sessions.Help()
	case tabAgents:
		return m.agents.Help()
	}
	return nil
}

// renderHelp monta o modal de ajuda (?) num Panel: grupo global de navegação +
// os grupos da aba ativa, arranjados em duas colunas (uma só em terminal estreito).
func (m Model) renderHelp() string {
	global := views.HelpGroup{Title: "Navegação", Keys: [][2]string{
		{"tab", "próxima aba"},
		{"shift+tab", "aba anterior"},
		{":", "paleta de comandos"},
		{"?", "fecha a ajuda"},
		{"q", "sair"},
	}}
	groups := append([]views.HelpGroup{global}, m.activeHelp()...)

	title := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	desc := lipgloss.NewStyle().Foreground(colorSubtle)

	blocks := make([]string, 0, len(groups))
	for _, g := range groups {
		caps := make([]string, len(g.Keys))
		maxCap := 0
		for i, kv := range g.Keys {
			caps[i] = components.Keycap(kv[0])
			if w := lipgloss.Width(caps[i]); w > maxCap {
				maxCap = w
			}
		}
		var b strings.Builder
		b.WriteString(title.Render(g.Title) + "\n")
		for i, kv := range g.Keys {
			pad := maxCap - lipgloss.Width(caps[i]) + 1
			b.WriteString("  " + caps[i] + strings.Repeat(" ", pad) + desc.Render(kv[1]) + "\n")
		}
		blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
	}

	w := m.width - 4
	if w > 74 {
		w = 74
	}
	content := helpColumns(blocks, w >= 60)
	return components.Panel{
		Title:   "Ajuda — " + tabNames[m.active],
		Focused: true,
		Width:   w,
	}.Render(content)
}

// helpColumns empilha os blocos em duas colunas (com uma linha em branco entre
// blocos da mesma coluna) ou numa só quando twoCol é falso (terminal estreito).
func helpColumns(blocks []string, twoCol bool) string {
	stack := func(bs []string) string {
		spaced := make([]string, 0, len(bs)*2)
		for i, b := range bs {
			if i > 0 {
				spaced = append(spaced, "")
			}
			spaced = append(spaced, b)
		}
		return lipgloss.JoinVertical(lipgloss.Left, spaced...)
	}
	if !twoCol || len(blocks) <= 1 {
		return stack(blocks)
	}
	half := (len(blocks) + 1) / 2
	return lipgloss.JoinHorizontal(lipgloss.Top, stack(blocks[:half]), "    ", stack(blocks[half:]))
}
