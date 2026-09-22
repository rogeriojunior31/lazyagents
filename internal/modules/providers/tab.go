package providers

import (
	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// Tab é a aba de provedores: a matriz perfil × agente e a aplicação de um
// perfil na config viva de cada CLI. Toda escrita passa por um confirm que
// diz qual arquivo vai ser reescrito. Semântica de ponteiro (module.Module).
type Tab struct {
	svc      *Service
	profiles []agent.ProviderProfile
	statuses []Status

	cursor  int
	confirm *components.Confirm
	action  func() tea.Msg // o que rodar quando o confirm der Yes

	loaded, loading bool
	width, height   int
	toast           string
	toastErr        bool
}

func newTab(svc *Service) Tab { return Tab{svc: svc} }

func (m Tab) Init() tea.Cmd   { return nil } // carga só ao abrir a aba
func (m *Tab) ID() string     { return "providers" }
func (m *Tab) Title() string  { return "Provedores" }
func (m Tab) Count() int      { return len(m.profiles) }
func (m Tab) Capturing() bool { return m.confirm != nil }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd lê a biblioteca de perfis e o que está aplicado em cada agente.
// Roda fora da thread de render: Status detecta agentes (`--version`) e lê os
// arquivos vivos.
func (m *Tab) loadCmd() tea.Cmd {
	svc := m.svc
	m.loading = true
	return func() tea.Msg {
		profiles, err := svc.Profiles()
		return loadedMsg{profiles: profiles, statuses: svc.Status(), err: err}
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
		m.profiles, m.statuses = msg.profiles, msg.statuses
		m.cursor = min(m.cursor, max(0, len(m.profiles)-1))
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
		}

	case doneMsg:
		m.toast, m.toastErr = msg.text, msg.err
		return m.loadCmd()

	case clearAllMsg: // paleta
		return m.ask("Remover o provedor de todos os agentes instalados?", func() tea.Msg {
			return m.done(m.svc.Clear(""), "provedor removido de todos os agentes")
		})

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
	case "x":
		return m.ask("Remover o provedor de todos os agentes instalados?", func() tea.Msg {
			return m.done(m.svc.Clear(""), "provedor removido de todos os agentes")
		})
	case "d":
		p, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask("Apagar o perfil "+p.Name+"? (os agentes onde ele foi aplicado não são tocados)", func() tea.Msg {
			return m.done(m.svc.Delete(p.Name), "perfil "+p.Name+" apagado")
		})
	case "space", "a":
		p, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask("Aplicar o perfil "+p.Name+" em todos os agentes instalados? Os arquivos de config são reescritos (com backup).", func() tea.Msg {
			return m.done(m.svc.Apply(p.Name, ""), "perfil "+p.Name+" aplicado em todos")
		})
	}
	// 1-9: alterna o perfil no N-ésimo agente da matriz.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return m.toggleAgent(int(key[0] - '1'))
	}
	return nil
}

// toggleAgent aplica o perfil selecionado no agente i, ou o remove se ele já
// é o que está aplicado lá.
func (m *Tab) toggleAgent(i int) tea.Cmd {
	p, ok := m.current()
	if !ok || i >= len(m.statuses) {
		return nil
	}
	st := m.statuses[i]
	if !st.Installed {
		m.toast, m.toastErr = st.AgentID+" não está instalado", true
		return nil
	}
	if st.Profile == p.Name {
		return m.ask("Remover o provedor de "+st.AgentName+"? Reescreve "+st.File+" (com backup).", func() tea.Msg {
			return m.done(m.svc.Clear(st.AgentID), "provedor removido de "+st.AgentID)
		})
	}
	from := "padrão do agente"
	if st.Active {
		from = st.Applied.BaseURL
	}
	return m.ask("Aplicar "+p.Name+" em "+st.AgentName+"?\n"+from+"  →  "+p.BaseURL+"\nReescreve "+st.File+" (com backup).", func() tea.Msg {
		return m.done(m.svc.Apply(p.Name, st.AgentID), "perfil "+p.Name+" aplicado em "+st.AgentID)
	})
}

// ask arma o confirm; a ação só roda no Yes.
func (m *Tab) ask(question string, action func() tea.Msg) tea.Cmd {
	c := components.NewConfirm(question)
	m.confirm, m.action = &c, action
	return nil
}

// done vira o toast do resultado de uma escrita.
func (m *Tab) done(err error, ok string) doneMsg {
	if err != nil {
		return doneMsg{text: err.Error(), err: true}
	}
	return doneMsg{text: ok}
}

func (m *Tab) current() (agent.ProviderProfile, bool) {
	if m.cursor < 0 || m.cursor >= len(m.profiles) {
		return agent.ProviderProfile{}, false
	}
	return m.profiles[m.cursor], true
}

func (m *Tab) move(d int) {
	if len(m.profiles) == 0 {
		return
	}
	m.cursor = max(0, min(len(m.profiles)-1, m.cursor+d))
}
