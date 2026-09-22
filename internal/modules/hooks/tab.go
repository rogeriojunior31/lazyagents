package hooks

import (
	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
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

	cursor  int
	confirm *components.Confirm
	action  func() tea.Msg

	loaded, loading bool
	width, height   int
	toast           string
	toastErr        bool
}

func newTab(svc *Service) Tab { return Tab{svc: svc} }

func (m Tab) Init() tea.Cmd   { return nil } // carga só ao abrir a aba
func (m *Tab) ID() string     { return "hooks" }
func (m *Tab) Title() string  { return "Hooks" }
func (m Tab) Count() int      { return len(m.lib) }
func (m Tab) Capturing() bool { return m.confirm != nil }
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

	case loadedMsg:
		m.loading = false
		m.lib, m.problems, m.statuses = msg.lib, msg.problems, msg.statuses
		m.cursor = min(m.cursor, max(0, len(m.lib)-1))
		if len(msg.problems) > 0 {
			m.toast, m.toastErr = msg.problems[0], true
		}

	case doneMsg:
		m.toast, m.toastErr = msg.text, msg.err
		return m.loadCmd()

	case tea.KeyPressMsg:
		return m.key(msg)

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
	switch key {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "r":
		m.toast = ""
		return m.loadCmd()
	case "space", "a":
		h, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask("Instalar o hook "+h.Name+" em todos os agentes que disparam "+h.Event+"?\n"+h.Command+"\nOs arquivos de config são reescritos (com backup).", func() tea.Msg {
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
	if enabledIn(st, h.Name) {
		return m.ask("Remover o hook "+h.Name+" de "+st.AgentName+"?\nReescreve "+st.File+" (com backup).", func() tea.Msg {
			return done(m.svc.Disable(h.Name, st.AgentID), "hook "+h.Name+" removido de "+st.AgentID)
		})
	}
	question := "Instalar o hook " + h.Name + " em " + st.AgentName + "?\n" +
		h.Event + " → " + h.Command + "\nReescreve " + st.File + " (com backup)."
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

func enabledIn(st Status, name string) bool {
	for _, n := range st.Enabled {
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
	m.cursor = max(0, min(len(m.lib)-1, m.cursor+d))
}
