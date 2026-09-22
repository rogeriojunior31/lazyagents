// Package tui é o root Bubble Tea do lazyagents: splash, header, abas, ajuda e
// paleta. As abas são module.Module registrados em internal/app — este arquivo
// não conhece nenhuma aba concreta. Todo I/O acontece nos services, em tea.Cmd.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
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

// tabLabel monta o texto da aba com o contador do módulo.
func (m Model) tabLabel(i int) string {
	mod := m.mods[i]
	if n := mod.Count(); n >= 0 {
		return fmt.Sprintf("%s · %d", mod.Title(), n)
	}
	return mod.Title()
}

// renderPill keeps navigation labels within the available terminal width.
func (m Model) renderPill(i int) string {
	label := m.tabLabel(i)
	if m.width < 80 {
		label = m.mods[i].Title()
	}
	w := max(1, m.width/max(1, len(m.mods)))
	style := m.styles.pill
	if i == m.active {
		style = m.styles.pillOn
	}
	if m.width < 80 {
		style = style.Padding(0, 1)
	}
	return style.MaxWidth(w).Render(label)
}

// Offsets do layout do View(), usados para traduzir cliques do mouse:
// masthead de três linhas, navegação, corpo com Padding(1,2).
const (
	tabRowY     = 2
	bodyOriginY = 4 // header (0) + respiro (1) + abas (2) + padding-top do body
	bodyOriginX = 2 // padding-left do body
)

// bodyWidth/bodyHeight são a área entregue aos módulos: descontam o padding
// lateral do body e as linhas de header, respiro, abas e padding vertical.
func bodyWidth(w int) int  { return max(1, w-4) }
func bodyHeight(h int) int { return max(1, h-5) }

// Model é o root: roteia teclas para a aba ativa e faz broadcast de mensagens
// assíncronas para todos os módulos.
type Model struct {
	keys     keyMap
	styles   styles
	help     help.Model
	adapters []agent.Adapter

	version     string
	state       appState
	splash      components.Splash
	showHelp    bool // modal de ajuda (?) aberto sobre a aba ativa
	showPalette bool // paleta de comandos (:) aberta sobre a aba ativa
	palette     components.Palette
	paletteIdx  map[string]paletteEntry
	mods        []module.Module
	active      int
	width       int
	height      int
}

// paletteEntry é o destino de um comando da paleta: módulo-alvo (-1 = global)
// e mensagem entregue a ele (nil = só troca de aba).
type paletteEntry struct {
	mod int
	msg tea.Msg
}

// buildPalette monta o catálogo da paleta: uma entrada por módulo (troca de
// aba), os comandos de cada module.Commander e os globais.
func buildPalette(mods []module.Module) ([]components.Command, map[string]paletteEntry) {
	var cmds []components.Command
	idx := map[string]paletteEntry{}
	add := func(name, desc string, e paletteEntry) {
		if _, dup := idx[name]; dup {
			return
		}
		cmds = append(cmds, components.Command{Name: name, Desc: desc})
		idx[name] = e
	}
	for i, mod := range mods {
		add(mod.ID(), "abre a aba "+mod.Title(), paletteEntry{mod: i})
	}
	for i, mod := range mods {
		if c, ok := mod.(module.Commander); ok {
			for _, pc := range c.Commands() {
				add(mod.ID()+" "+pc.Name, pc.Desc, paletteEntry{mod: i, msg: pc.Msg})
			}
		}
	}
	add("help", "abre a ajuda da aba atual", paletteEntry{mod: -1})
	add("reload", "recarrega a aba atual", paletteEntry{mod: -1})
	add("quit", "sai do lazyagents", paletteEntry{mod: -1})
	return cmds, idx
}

// New monta o root com os módulos na ordem das abas.
func New(mods []module.Module, adapters []agent.Adapter, version string) Model {
	cmds, idx := buildPalette(mods)
	return Model{
		keys:       newKeyMap(),
		styles:     newStyles(),
		help:       newHelp(),
		adapters:   adapters,
		version:    version,
		state:      stateSplash,
		splash:     components.NewSplash(version),
		palette:    components.NewPalette(cmds),
		paletteIdx: idx,
		mods:       mods,
	}
}

// detectCmd roda a detecção dos agentes (inclui --version de cada CLI) fora do
// Update; o resultado alimenta todos os módulos.
func (m Model) detectCmd() tea.Cmd {
	adapters := m.adapters
	return func() tea.Msg {
		return events.AgentsDetected{Agents: agent.DetectAll(adapters)}
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

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.detectCmd(), splashTimerCmd()}
	for _, mod := range m.mods {
		cmds = append(cmds, mod.Init())
	}
	return tea.Batch(cmds...)
}

func (m Model) capturingInput() bool {
	if len(m.mods) == 0 {
		return false
	}
	return m.mods[m.active].Capturing()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// splash: só Enter/espaço/timer avançam; mensagens async passam para as views
	// para que skills e sessões carreguem em background durante o splash.
	if m.state == stateSplash {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width, m.height = msg.Width, msg.Height
			m.splash = m.splash.Resize(msg.Width, msg.Height)
			inner := tea.WindowSizeMsg{Width: bodyWidth(msg.Width), Height: bodyHeight(msg.Height)}
			return m.updateViews(inner)
		case splashDoneMsg:
			m.state = stateMain
			return m, m.switchTo(m.active)
		case tea.KeyPressMsg:
			if msg.String() == "enter" || msg.String() == "space" {
				m.state = stateMain
				return m, m.switchTo(m.active)
			}
			return m, nil
		default:
			// events.AgentsDetected, SkillsScanned, SessionsLoaded etc. — deixar as views processar
			// em background enquanto o splash está visível.
			return m.updateViews(msg)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		inner := tea.WindowSizeMsg{Width: bodyWidth(msg.Width), Height: bodyHeight(msg.Height)}
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
			case key.Matches(msg, m.keys.NextTab) && len(m.mods) > 0:
				return m, m.switchTo((m.active + 1) % len(m.mods))
			case key.Matches(msg, m.keys.PrevTab) && len(m.mods) > 0:
				return m, m.switchTo((m.active + len(m.mods) - 1) % len(m.mods))
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
			for i := range m.mods {
				w := lipgloss.Width(m.renderPill(i))
				if msg.X >= x && msg.X < x+w {
					return m, m.switchTo(i)
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

// switchTo ativa o módulo i, some com os toasts e avisa quem faz carga sob
// demanda (events.TabActivated chega a todos pelo broadcast).
func (m *Model) switchTo(i int) tea.Cmd {
	m.active = i
	for _, mod := range m.mods {
		mod.ClearToast()
	}
	if i < 0 || i >= len(m.mods) {
		return nil
	}
	id := m.mods[i].ID()
	return func() tea.Msg { return events.TabActivated{ID: id} }
}

// runPaletteCommand executa o comando escolhido na paleta.
func (m Model) runPaletteCommand(name string) (tea.Model, tea.Cmd) {
	e, ok := m.paletteIdx[name]
	if !ok {
		return m, nil
	}
	if e.mod >= 0 {
		activated := m.switchTo(e.mod)
		if e.msg != nil {
			return m, tea.Batch(activated, m.mods[e.mod].Update(e.msg))
		}
		return m, activated
	}
	switch name {
	case "help":
		m.showHelp = true
	case "quit":
		return m, tea.Quit
	case "reload":
		return m.updateActive(events.Reload{})
	}
	return m, nil
}

// updateActive entrega msg só ao módulo ativo (teclado, mouse, paste).
func (m Model) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.mods) == 0 {
		return m, nil
	}
	return m, m.mods[m.active].Update(msg)
}

// updateViews faz broadcast de msg para todos os módulos (tamanho, events.*,
// resultados assíncronos) — cada módulo ignora o que não é dele.
func (m Model) updateViews(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, len(m.mods))
	for _, mod := range m.mods {
		cmds = append(cmds, mod.Update(msg))
	}
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
	for i := range m.mods {
		tabs = append(tabs, m.renderPill(i))
	}

	// Header: marca à esquerda, atalhos globais à direita. Os contadores já
	// estão nas abas; o rodapé é das dicas contextuais de cada módulo.
	left := m.styles.badge.Render("◈  lazyagents")
	if m.width >= 100 {
		left += m.styles.tagline.Render("A G E N T   W O R K S P A C E")
	}
	right := m.help.View(m.keys)
	if lipgloss.Width(left)+lipgloss.Width(right)+4 > m.width {
		right = ""
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2)
	header := ansi.Truncate(left+strings.Repeat(" ", gap)+right, m.width, "")

	var body string
	switch {
	case m.showPalette:
		body = m.renderPalette()
	case m.showHelp:
		body = m.renderHelp()
	case len(m.mods) > 0:
		body = m.mods[m.active].View()
	}

	// Reserve the body area so dialogs and short pages keep the hints anchored.
	bodyW, bodyH := bodyWidth(m.width), bodyHeight(m.height)
	if m.showHelp || m.showPalette {
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, body)
	}
	body = lipgloss.NewStyle().Width(bodyW).Height(bodyH).
		MaxWidth(bodyW).MaxHeight(bodyH).Render(body)
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		ansi.Truncate(lipgloss.JoinHorizontal(lipgloss.Top, tabs...), m.width, "…"),
		m.styles.body.Render(body),
	)
	v.Content = lipgloss.NewStyle().Foreground(theme.Text).Background(theme.Bg).
		Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height).Render(content)
	v.Content = theme.Paint(v.Content, theme.Text, theme.Bg)
	// O fundo do terminal vira o Bg do tema: célula que escapar da pintura
	// (resize, clear) nunca mostra a cor padrão do emulador.
	v.BackgroundColor = theme.Bg
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
func (m Model) activeHelp() []module.HelpGroup {
	if len(m.mods) == 0 {
		return nil
	}
	return m.mods[m.active].Help()
}

// renderHelp monta o modal de ajuda (?) num Panel: grupo global de navegação +
// os grupos da aba ativa, arranjados em duas colunas (uma só em terminal estreito).
func (m Model) renderHelp() string {
	global := module.HelpGroup{Title: "Navegação", Keys: [][2]string{
		{"tab", "próxima aba"},
		{"shift+tab", "aba anterior"},
		{":", "paleta de comandos"},
		{"?", "fecha a ajuda"},
		{"q", "sair"},
	}}
	groups := append([]module.HelpGroup{global}, m.activeHelp()...)

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

	// Largura pelo conteúdo: duas colunas quando cabem no corpo, senão uma.
	// Panel desconta 2 colunas de padding de cada lado.
	panelTitle := "Ajuda — " + m.activeTitle()
	maxW := bodyWidth(m.width)
	content := helpColumns(blocks, true)
	if lipgloss.Width(content)+4 > maxW {
		content = helpColumns(blocks, false)
	}
	w := min(maxW, max(lipgloss.Width(content), lipgloss.Width(panelTitle))+4)
	return components.Panel{
		Title:   panelTitle,
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

func (m Model) activeTitle() string {
	if len(m.mods) == 0 {
		return ""
	}
	return m.mods[m.active].Title()
}
