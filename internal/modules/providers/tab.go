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
	col       int // agente sob o cursor na matriz (índice em statuses)
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
func (m *Tab) Title() string  { return "Providers" }
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
		m.col = max(0, min(m.col, len(m.statuses)-1))
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
		}

	case doneMsg:
		m.toast, m.toastErr = msg.text, msg.err
		return m.loadCmd()

	case newProfileMsg: // paleta
		m.form = newProfileForm(agent.ProviderProfile{}, false)

	case clearAllMsg: // paleta
		return m.ask("Clear the provider from every installed agent?", func() tea.Msg {
			return m.done(m.svc.Clear(""), "provider cleared from all agents")
		})

	case savedMsg:
		if msg.err != nil {
			if m.form != nil { // erro de validação: o formulário continua aberto
				m.form.err = msg.err.Error()
			}
			return nil
		}
		m.form = nil
		m.toast, m.toastErr = fmt.Sprintf("profile %s saved", msg.name), false
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
		if msg.Button != tea.MouseLeft {
			return nil
		}
		sp, top := m.split(), m.inUseHeight()
		x, y := msg.X, msg.Y-top
		if y < 0 || sp.Side && x >= sp.ListW || !sp.Side && y >= sp.ListH {
			return nil // "em uso" e detalhe: só leitura
		}
		start, end := kit.Window(m.cursor, len(m.profiles), max(1, sp.ListH-profilesTop))
		if i := start + y - profilesTop; y >= profilesTop && i < end {
			m.move(i - m.cursor)
			if c := kit.ColumnAt(sp.ListW, m.tableCols(sp.ListW), x) - colAgents; c >= 0 && c < len(m.statuses) {
				m.col = c // clique na célula escolhe o agente; space alterna
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
		if sp := m.split(); sp.Side && msg.X >= sp.ListW || !sp.Side && msg.Y >= m.inUseHeight()+sp.ListH {
			m.scrollDetail(msg) // roda sobre o detalhe rola ele
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
			m.toast, m.toastErr = "applying…", false
			return func() tea.Msg { return action() }
		case components.No:
			m.confirm, m.action = nil, nil
			m.toast = ""
		}
		return nil
	}

	key := msg.String()
	if kit.DetailScroll[key] {
		m.scrollDetail(kit.DetailScrollMsg(msg))
		return nil
	}
	switch key {
	case "left", "h":
		m.col = max(0, m.col-1)
	case "right", "l":
		m.col = max(0, min(len(m.statuses)-1, m.col+1))
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "r":
		m.toast = ""
		return m.loadCmd()
	case "x":
		return m.ask("Clear the provider from every installed agent?", func() tea.Msg {
			return m.done(m.svc.Clear(""), "provider cleared from all agents")
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
		return m.ask(fmt.Sprintf("Delete profile %s? (agents where it was applied are not touched)", p.Name), func() tea.Msg {
			return m.done(m.svc.Delete(p.Name), fmt.Sprintf("profile %s deleted", p.Name))
		})
	case "space":
		return m.toggleAgent(m.col)
	case "a":
		p, ok := m.current()
		if !ok {
			return nil
		}
		return m.ask(fmt.Sprintf("Apply profile %s to every installed agent? Their config files are rewritten (with a backup).", p.Name), func() tea.Msg {
			return m.done(m.svc.Apply(p.Name, ""), fmt.Sprintf("profile %s applied to all agents", p.Name))
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
				return savedMsg{err: fmt.Errorf("profile %q already exists (e edits it)", p.Name)}
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
		m.toast, m.toastErr = fmt.Sprintf("%s is not installed", st.AgentID), true
		return nil
	}
	if st.Profile == p.Name {
		return m.ask(fmt.Sprintf("Clear the provider from %s?\nRewrites %s (with a backup).", st.AgentName, core.Tilde(st.File, m.svc.home)), func() tea.Msg {
			return m.done(m.svc.Clear(st.AgentID), fmt.Sprintf("provider cleared from %s", st.AgentID))
		})
	}
	from := "agent default"
	if st.Active {
		from = st.Applied.BaseURL
	}
	return m.ask(fmt.Sprintf("Apply %s to %s?\n%s  →  %s\nRewrites %s (with a backup).", p.Name, st.AgentName, from, p.BaseURL, core.Tilde(st.File, m.svc.home)), func() tea.Msg {
		return m.done(m.svc.Apply(p.Name, st.AgentID), fmt.Sprintf("profile %s applied to %s", p.Name, st.AgentID))
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
