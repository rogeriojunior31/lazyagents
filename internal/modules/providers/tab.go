package providers

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// Tab é a aba de provedores: a matriz perfil × agente e a aplicação de um
// perfil na config viva de cada CLI. Toda escrita passa por um confirm que
// diz qual arquivo vai ser reescrito. Semântica de ponteiro (module.Module).
type Tab struct {
	svc      *Service
	profiles []agent.ProviderProfile
	statuses []Status

	cursor    int
	paneFocus kit.PaneID
	detailOff int
	confirm   *components.Confirm
	form      *profileForm   // criar/editar perfil; dono do teclado quando aberto
	action    func() tea.Msg // o que rodar quando o confirm der Yes

	loaded, loading bool
	width, height   int
	toast           string
	toastErr        bool
}

func newTab(svc *Service) Tab { return Tab{svc: svc} }

// Init lê só os perfis (redigidos), para o contador da pill não mentir antes
// de a aba abrir; o estado nos agentes espera a ativação.
func (m Tab) Init() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		profiles, _ := svc.Profiles()
		return profilesMsg{profiles: redacted(profiles)}
	}
}

func (m *Tab) ID() string     { return "providers" }
func (m *Tab) Title() string  { return "Provedores" }
func (m Tab) Count() int      { return len(m.profiles) }
func (m Tab) Capturing() bool { return m.confirm != nil || m.form != nil }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd lê a biblioteca de perfis e o que está aplicado em cada agente.
// Roda fora da thread de render: Status detecta agentes (`--version`) e lê os
// arquivos vivos.
func (m *Tab) loadCmd() tea.Cmd {
	svc := m.svc
	m.loading = true
	return func() tea.Msg {
		profiles, err := svc.Profiles()
		return loadedMsg{profiles: redacted(profiles), statuses: svc.Status(), err: err}
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

	case profilesMsg:
		if !m.loaded {
			m.profiles = msg.profiles
		}

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

	case newProfileMsg: // paleta
		m.form = newProfileForm(agent.ProviderProfile{}, false)

	case clearAllMsg: // paleta
		return m.ask("Remover o provedor de todos os agentes instalados?", func() tea.Msg {
			return m.done(m.svc.Clear(""), "provedor removido de todos os agentes")
		})

	case savedMsg:
		if msg.err != nil {
			if m.form != nil { // erro de validação: o formulário continua aberto
				m.form.err = msg.err.Error()
			}
			return nil
		}
		m.form = nil
		m.toast, m.toastErr = "perfil "+msg.name+" salvo", false
		return m.loadCmd()

	case tea.PasteMsg:
		if m.form != nil {
			return m.form.paste(msg)
		}

	case tea.KeyPressMsg:
		if m.form != nil {
			return m.formKey(msg)
		}
		return m.key(msg)

	case tea.MouseClickMsg:
		if m.confirm != nil || m.form != nil {
			return nil
		}
		if msg.Button == tea.MouseLeft && m.width >= narrowWidth && msg.X >= m.listWidth()+2 && msg.Y >= 0 && msg.Y < m.bodyHeight() {
			m.paneFocus = kit.PaneDetail
			return nil
		}
		if m.width < narrowWidth && m.paneFocus == kit.PaneDetail {
			return nil
		}
		if msg.Button == tea.MouseLeft && msg.X < m.listWidth() && msg.Y < m.listHeight() {
			if i := kit.RowAt(msg.Y, m.cursor, len(m.profiles), m.listHeight()); i >= 0 {
				m.move(i - m.cursor)
				m.paneFocus = kit.PaneList
			}
		}

	case tea.MouseWheelMsg:
		if m.form != nil {
			return nil
		}
		if m.confirm != nil {
			c, _ := m.confirm.Update(msg, m.width, m.height)
			m.confirm = &c
			return nil
		}
		if m.paneFocus == kit.PaneDetail {
			m.scrollDetail(msg)
			return nil
		}
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
		c, res := m.confirm.Update(msg, m.width, m.height)
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
	if m.paneFocus == kit.PaneDetail && kit.DetailScrollKeys[key] {
		m.scrollDetail(msg)
		return nil
	}
	switch key {
	case "left":
		m.paneFocus = kit.PaneList
	case "right":
		m.paneFocus = kit.PaneDetail
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
	case "n":
		m.form = newProfileForm(agent.ProviderProfile{}, false)
		return nil
	case "e":
		if p, ok := m.current(); ok {
			m.form = newProfileForm(p, true)
		}
		return nil
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

// formKey encaminha a tecla ao formulário e salva no enter, fora da thread
// de render; o token digitado vai direto para o service.
func (m *Tab) formKey(msg tea.KeyPressMsg) tea.Cmd {
	res, cmd := m.form.update(msg)
	switch res {
	case formCancel:
		m.form = nil
	case formSubmit:
		svc, p, orig := m.svc, m.form.profile(), m.form.orig
		m.form.err = ""
		return func() tea.Msg {
			if orig != "" {
				return savedMsg{name: p.Name, err: svc.Edit(orig, p)}
			}
			if _, err := svc.Profile(p.Name); err == nil {
				return savedMsg{err: fmt.Errorf("já existe um perfil %q (e edita)", p.Name)}
			}
			return savedMsg{name: p.Name, err: svc.Save(p)}
		}
	}
	return cmd
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
		return m.ask("Remover o provedor de "+st.AgentName+"?\nReescreve "+core.Tilde(st.File, m.svc.home)+" (com backup).", func() tea.Msg {
			return m.done(m.svc.Clear(st.AgentID), "provedor removido de "+st.AgentID)
		})
	}
	from := "padrão do agente"
	if st.Active {
		from = st.Applied.BaseURL
	}
	return m.ask("Aplicar "+p.Name+" em "+st.AgentName+"?\n"+from+"  →  "+p.BaseURL+"\nReescreve "+core.Tilde(st.File, m.svc.home)+" (com backup).", func() tea.Msg {
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
	next := max(0, min(len(m.profiles)-1, m.cursor+d))
	if next != m.cursor {
		m.detailOff = 0
	}
	m.cursor = next
}

// redacted tira os tokens: a aba só exibe e aplica por nome, então o segredo
// nem chega ao model.
func redacted(profiles []agent.ProviderProfile) []agent.ProviderProfile {
	for i := range profiles {
		profiles[i] = profiles[i].Redacted()
	}
	return profiles
}
