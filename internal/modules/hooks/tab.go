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

// Tab é a aba de hooks: a matriz hook × agente e a instalação de um hook na
// config viva de cada CLI. Como toda escrita em arquivo de agente, passa por
// um confirm que diz o que vai ser reescrito. Semântica de ponteiro
// (module.Module).
type Tab struct {
	svc      *Service
	lib      []Hook
	problems []string
	statuses []Status

	cursor    int
	detailOff int // rolagem do painel de detalhe
	// cmdMode põe o teclado na lista de comandos da entrada (enter), para
	// ligar e desligar um a um; cmdCursor é a posição em commandOrder.
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

// Init lê só a biblioteca, para o contador da pill não mentir antes de a aba
// abrir; o estado nos agentes (que detecta os CLIs) espera a ativação.
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
func (m Tab) Capturing() bool { return m.confirm != nil || m.cmdMode }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd lê a biblioteca e o que está instalado em cada agente. Roda fora da
// thread de render: Status detecta agentes e lê os arquivos vivos.
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

	case events.TabActivated:
		if msg.ID == m.ID() && !m.loaded && !m.loading {
			m.loaded = true
			return m.loadCmd()
		}

	case events.Reload:
		m.loaded = true
		return m.loadCmd()

	case libraryMsg:
		if !m.loaded { // a carga completa já trouxe a biblioteca
			m.lib = msg.lib
		}

	case loadedMsg:
		m.loading = false
		m.lib, m.problems, m.statuses = msg.lib, msg.problems, msg.statuses
		m.cursor = min(m.cursor, max(0, len(m.lib)-1))
		if h, ok := m.current(); !ok || m.cmdCursor >= len(h.Hooks) {
			m.cmdMode, m.cmdCursor = false, 0
		}
		if len(msg.problems) > 0 {
			m.toast, m.toastErr = msg.problems[0], true
		}

	case doneMsg:
		m.toast, m.toastErr = msg.text, msg.err
		return m.loadCmd()

	case tea.KeyPressMsg:
		return m.key(msg)

	case tea.MouseClickMsg:
		if m.cmdMode {
			return nil
		}
		if msg.Button == tea.MouseLeft && msg.X < m.listWidth() && msg.Y < m.listHeight() {
			if i := kit.RowAt(msg.Y, m.cursor, len(m.lib), m.listHeight()); i >= 0 {
				m.move(i - m.cursor)
			}
		}

	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			m.move(-1)
		} else if msg.Button == tea.MouseWheelDown {
			m.move(1)
		}
	}
	return nil
}

func (m *Tab) key(msg tea.KeyPressMsg) tea.Cmd {
	if m.confirm != nil {
		c, res := m.confirm.Update(msg)
		m.confirm = &c
		switch res {
		case components.Yes:
			action := m.action
			m.confirm, m.action = nil, nil
			m.toast, m.toastErr = "aplicando…", false
			return func() tea.Msg { return action() }
		case components.No:
			m.confirm, m.action = nil, nil
			m.toast = ""
		}
		return nil
	}

	key := msg.String()
	if m.cmdMode {
		return m.cmdKey(key)
	}
	switch key {
	case "enter", "right", "l":
		if h, ok := m.current(); ok && len(h.Hooks) > 0 {
			m.cmdMode, m.cmdCursor = true, 0
			m.followCmd()
		}
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgdown", "ctrl+d":
		m.detailOff = min(m.maxDetailOff(), m.detailOff+max(1, m.bodyHeight()/2))
	case "pgup", "ctrl+u":
		m.detailOff = max(0, m.detailOff-max(1, m.bodyHeight()/2))
	case "r":
		m.toast = ""
		return m.loadCmd()
	case "space", "a":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask("Instalar o hook "+h.Name+" em todos os agentes que disparam "+strings.Join(h.Events(), ", ")+"?\n"+h.Summary()+"\nOs arquivos de config são reescritos (com backup).", func() tea.Msg {
			return done(m.svc.Enable(h.Name, ""), "hook "+h.Name+" instalado em todos")
		})
	case "x":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask("Remover o hook "+h.Name+" de todos os agentes?", func() tea.Msg {
			return done(m.svc.Disable(h.Name, ""), "hook "+h.Name+" removido de todos")
		})
	case "d":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask("Apagar o hook "+h.Name+" da biblioteca? (onde já está instalado, ele continua)", func() tea.Msg {
			return done(m.svc.Delete(h.Name), "hook "+h.Name+" apagado da biblioteca")
		})
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return m.toggleAgent(int(key[0] - '1'))
	}
	return nil
}

// cmdKey trata o teclado na lista de comandos da entrada.
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
		m.followCmd()
	case "down", "j":
		m.cmdCursor = min(len(h.Hooks)-1, m.cmdCursor+1)
		m.followCmd()
	case "space", "enter":
		return m.toggleCommand(h)
	}
	return nil
}

// toggleCommand liga ou desliga o comando sob o cursor. Sem a entrada
// instalada em lugar nenhum, só a biblioteca muda e não há o que confirmar;
// com ela instalada, a mudança reescreve o arquivo do agente.
func (m *Tab) toggleCommand(h Hook) tea.Cmd {
	order := commandOrder(h)
	if m.cmdCursor >= len(order) {
		return nil
	}
	i := order[m.cmdCursor]
	c := h.Hooks[i]
	on := h.IsOff(i)
	verb, label := "desligado", "Desligar"
	if on {
		verb, label = "ligado", "Ligar"
	}
	svc, name := m.svc, h.Name
	action := func() tea.Msg {
		return done(svc.SetCommand(name, i, on), fmt.Sprintf("%s · %s %s", name, commandLabel(c), verb))
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
	return m.ask(fmt.Sprintf("%s %s · %s em %s?\n%s\nReescreve %s (com backup).",
		label, c.Event, commandLabel(c), name, displayCommand(c.Command), strings.Join(where, ", ")), action)
}

// followCmd rola o detalhe para o comando sob o cursor ficar visível.
func (m *Tab) followCmd() {
	p := components.Panel{Width: m.detailWidth(), Height: m.detailHeight()}
	lines := strings.Split(m.detailContent(p.ContentWidth()), "\n")
	sel := -1
	for i, l := range lines {
		if strings.Contains(l, cmdCursorMark) {
			sel = i
			break
		}
	}
	if sel < 0 {
		return
	}
	visible := max(1, p.ContentHeight()-1) // a última linha pode virar "↓ mais"
	if sel < m.detailOff {
		m.detailOff = sel
	} else if sel >= m.detailOff+visible {
		m.detailOff = sel - visible + 1
	}
}

// toggleAgent instala o hook no agente i, ou o remove se já estiver lá.
func (m *Tab) toggleAgent(i int) tea.Cmd {
	h, ok := m.current()
	if !ok || i >= len(m.statuses) {
		return nil
	}
	st := m.statuses[i]
	if !st.Installed {
		m.toast, m.toastErr = st.AgentID+" não está instalado", true
		return nil
	}
	if enabledIn(st, h.Name) || partialIn(st, h.Name) {
		return m.ask("Remover o hook "+h.Name+" de "+st.AgentName+"?\nReescreve "+core.Tilde(st.File, m.svc.home)+" (com backup).", func() tea.Msg {
			return done(m.svc.Disable(h.Name, st.AgentID), "hook "+h.Name+" removido de "+st.AgentID)
		})
	}
	question := "Instalar o hook " + h.Name + " em " + st.AgentName + "?\n" +
		strings.Join(h.Events(), ", ") + " → " + h.Summary() + "\nReescreve " + core.Tilde(st.File, m.svc.home) + " (com backup)."
	if h.Imported() {
		question += "\nImportado de " + h.Source + " (feito para o Claude Code)."
	}
	if st.Note != "" {
		question += "\n" + st.Note
	}
	return m.ask(question, func() tea.Msg {
		return done(m.svc.Enable(h.Name, st.AgentID), "hook "+h.Name+" instalado em "+st.AgentID)
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
