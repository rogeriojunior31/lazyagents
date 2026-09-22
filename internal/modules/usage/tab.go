package usage

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// historyDays limita a varredura de transcripts: o detalhe é dos últimos dias,
// e assim a aba não lê o histórico inteiro do usuário a cada abertura.
const historyDays = 7

// Tab é a aba de consumo: quanto da assinatura já foi usado em cada janela
// (sessão e semana) e, como detalhe, os tokens dos transcripts por dia e por
// projeto. Nada é carregado no boot — só quando a aba é aberta. Semântica de
// ponteiro (module.Module).
type Tab struct {
	svc      *Service
	statuses []Status
	events   []agent.UsageEvent
	sessions []agent.Session

	loaded        bool // já carregou uma vez (evita rede a cada troca de aba)
	loading       bool
	scroll        int
	width, height int
	toast         string
	toastErr      bool
}

func newTab(svc *Service) Tab { return Tab{svc: svc} }

func (m Tab) Init() tea.Cmd   { return nil } // carga só ao abrir a aba
func (m *Tab) ID() string     { return "usage" }
func (m *Tab) Title() string  { return "Uso" }
func (m Tab) Count() int      { return -1 }
func (m Tab) Capturing() bool { return false }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd busca limites (cacheados) e os eventos recentes dos transcripts.
func (m *Tab) loadCmd(refresh bool) tea.Cmd {
	svc, sessions := m.svc, m.recent()
	m.loading = true
	status := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return statusMsg{statuses: svc.Status(ctx, refresh)}
	}
	agg := func() tea.Msg { return eventsMsg{events: svc.Events(sessions)} }
	return tea.Batch(status, agg)
}

// recent filtra as sessões da janela de histórico (as mais antigas não entram
// nem no bloco atual nem nos últimos dias).
func (m Tab) recent() []agent.Session {
	cut := time.Now().AddDate(0, 0, -historyDays)
	var out []agent.Session
	for _, s := range m.sessions {
		if s.MTime.After(cut) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case events.SessionsLoaded:
		m.sessions = msg.Sessions
		if m.loaded { // já visitada: reagrega com a lista nova, sem rede
			return m.loadCmd(false)
		}

	case events.TabActivated:
		if msg.ID == m.ID() && !m.loaded && !m.loading {
			m.loaded = true
			m.toast = "consultando limites…"
			return m.loadCmd(false)
		}

	case refreshMsg: // paleta: usage refresh
		m.loaded = true
		m.toast, m.toastErr = "atualizando limites…", false
		return m.loadCmd(true)

	case events.Reload:
		m.loaded = true
		m.toast, m.toastErr = "atualizando limites…", false
		return m.loadCmd(true)

	case statusMsg:
		m.loading = false
		m.statuses = msg.statuses
		m.toast = ""
		for _, st := range msg.statuses {
			if st.Err != "" {
				m.toast, m.toastErr = st.AgentID+": "+st.Err, true
			}
		}

	case eventsMsg:
		m.events = msg.events

	case tea.KeyPressMsg:
		switch msg.String() {
		case "r":
			m.toast, m.toastErr = "atualizando limites…", false
			return m.loadCmd(true)
		case "up", "k":
			m.scroll = max(0, m.scroll-1)
		case "down", "j":
			m.scroll++
		case "g", "home":
			m.scroll = 0
		}

	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			m.scroll = max(0, m.scroll-1)
		} else if msg.Button == tea.MouseWheelDown {
			m.scroll++
		}
	}
	return nil
}
