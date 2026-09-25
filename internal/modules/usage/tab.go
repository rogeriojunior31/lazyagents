package usage

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// Tab é a aba de consumo: quanto da assinatura já foi usado em cada janela
// (sessão e semana) e os tokens dos transcripts, filtráveis por período,
// agente, visão (dia, agente, projeto, modelo) e texto. Nada é carregado no
// boot — só quando a aba é aberta. Semântica de ponteiro (module.Module).
type Tab struct {
	svc      *Service
	statuses []Status
	events   []agent.UsageEvent // todo o histórico; os filtros recortam em memória
	sessions []agent.Session
	names    map[string]string // id → nome de exibição (events.AgentsDetected)

	f         filters
	filtering bool // input de texto (/) aberto: dono do teclado
	input     textinput.Model

	// corpo já renderizado: recalcular agrega o histórico inteiro, então só
	// acontece quando muda o que ele mostra (rolar só recorta linhas)
	bar   string // filtros fixos, memorizados junto ao corpo
	lines []string
	drawn renderKey
	gen   int // sobe a cada carga de limites ou eventos

	loaded        bool // já carregou uma vez (evita rede a cada troca de aba)
	loading       bool
	pending       int // consultas de limites e consumo ainda em andamento
	scroll        int
	width, height int
	toast         string
	toastErr      bool
}

func newTab(svc *Service, cfg config) Tab { return Tab{svc: svc, f: newFilters(cfg)} }

func (m Tab) Init() tea.Cmd   { return nil } // carga só ao abrir a aba
func (m *Tab) ID() string     { return "usage" }
func (m *Tab) Title() string  { return "Usage" }
func (m Tab) Count() int      { return -1 }
func (m Tab) Capturing() bool { return m.filtering }
func (m *Tab) ClearToast()    { m.toast = "" }

// loadCmd busca limites (cacheados) e os eventos de todas as sessões. Com o
// índice de transcripts, ler o histórico inteiro custa milissegundos depois
// da primeira vez, e o período vira só um recorte em memória.
func (m *Tab) loadCmd(refresh bool) tea.Cmd {
	svc, sessions := m.svc, m.sessions
	m.loading = true
	m.pending += 2
	status := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return statusMsg{statuses: svc.Status(ctx, refresh)}
	}
	agg := func() tea.Msg { return eventsMsg{events: svc.Events(sessions)} }
	return tea.Batch(status, agg)
}

// renderKey é tudo de que o corpo depende; o minuto mantém em dia as
// contagens regressivas (reset, bloco atual).
type renderKey struct {
	f       filters
	width   int
	gen     int
	minute  int64
	loading bool
	typing  bool // o filtro de texto sai da barra enquanto o input está aberto
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	cmd := m.update(msg)
	key := renderKey{m.f, m.width, m.gen, time.Now().Unix() / 60, m.loading, m.filtering}
	if m.lines == nil || key != m.drawn {
		m.lines, m.drawn = m.bodyLines(), key
		m.bar = lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.filterBar())
	}
	m.scroll = min(m.scroll, max(0, len(m.lines)-m.contentHeight())) // não rola além do fim
	return cmd
}

func (m *Tab) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case events.AgentsDetected:
		m.names = map[string]string{}
		for _, a := range msg.Agents {
			m.names[a.ID] = a.Name
		}
		m.gen++ // os rótulos mudam: redesenha

	case events.SessionsLoaded:
		m.sessions = msg.Sessions
		if m.loaded { // já visitada: reagrega com a lista nova, sem rede
			return m.loadCmd(false)
		}

	case events.TabActivated:
		if msg.ID == m.ID() && !m.loaded && !m.loading {
			m.loaded = true
			m.toast = "fetching limits…"
			return m.loadCmd(false)
		}

	case refreshMsg: // paleta: usage refresh
		return m.refresh()

	case events.Reload:
		return m.refresh()

	case filterMsg: // paleta: usage period/view/agent …
		msg.apply(&m.f)
		m.scroll = 0

	case statusMsg:
		m.gen++
		m.finishLoad()
		m.statuses = msg.statuses
		m.toast, m.toastErr = "", false
		warnings := 0
		for _, st := range msg.statuses {
			if st.Err != "" {
				warnings++
			}
		}
		if warnings > 0 {
			m.toast, m.toastErr = fmt.Sprintf("%d warning(s) · see Limits", warnings), true
		}

	case eventsMsg:
		m.finishLoad()
		m.gen++
		m.events = msg.events

	case tea.PasteMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}

	case tea.KeyPressMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateKeys(msg)

	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			m.scroll = max(0, m.scroll-1)
		} else if msg.Button == tea.MouseWheelDown {
			m.scroll++
		}
	}
	return nil
}

func (m *Tab) refresh() tea.Cmd {
	m.loaded = true
	m.toast, m.toastErr = "refreshing limits…", false
	return m.loadCmd(true)
}

// updateKeys trata as teclas fora do input de texto.
func (m *Tab) updateKeys(msg tea.KeyPressMsg) tea.Cmd {
	step := 1
	switch msg.String() {
	case "r":
		return m.refresh()
	case "P":
		step = -1
		fallthrough
	case "p":
		m.f.period = (m.f.period + step + len(periods)) % len(periods)
		m.scroll = 0
	case "A":
		step = -1
		fallthrough
	case "a":
		m.f.agent = cycle(m.f.agent, agentsIn(m.statuses, m.events), step)
		m.scroll = 0
	case "left", "h", "V":
		step = -1
		fallthrough
	case "right", "l", "v":
		m.f.view = (m.f.view + step + len(tabViews)) % len(tabViews)
		m.scroll = 0
	case "/":
		m.input = components.NewInput()
		m.input.Placeholder = fmt.Sprintf("filter by %s…", tabViews[m.f.view].label)
		m.input.SetValue(m.f.text)
		m.input.SetWidth(40)
		m.input.Focus()
		m.filtering = true
	case "esc":
		if m.f.text != "" {
			m.f.text = ""
		} else {
			m.f.agent = ""
		}
		m.scroll = 0
	case "up", "k":
		m.scroll = max(0, m.scroll-1)
	case "down", "j":
		m.scroll++
	case "pgup":
		m.scroll = max(0, m.scroll-max(1, m.contentHeight()))
	case "pgdown", "space":
		m.scroll += max(1, m.contentHeight())
	case "g", "home":
		m.scroll = 0
	}
	return nil
}

// updateFilter é o input de texto: filtra enquanto digita; enter fecha e
// mantém, esc fecha e limpa.
func (m *Tab) updateFilter(msg tea.Msg) tea.Cmd {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "esc":
			m.filtering, m.f.text = false, ""
			m.input.Blur()
			return nil
		case "enter":
			m.filtering = false
			m.input.Blur()
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.f.text = m.input.Value()
	m.scroll = 0
	return cmd
}

func (m *Tab) finishLoad() {
	m.pending = max(0, m.pending-1)
	m.loading = m.pending > 0
}
