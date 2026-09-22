package sessions

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/session"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type sessMode int

const (
	sessModeList   sessMode = iota
	sessModeDoc             // lendo o transcript de uma sessão
	sessModeDir             // input de pasta para o resume
	sessModeSearch          // input de busca full-text nos transcripts
	sessModeAlias           // input de apelido da sessão (tecla m)
)

// Sessions é a aba de sessões unificadas de todos os agentes. enter suspende a
// TUI e retoma a sessão no CLI de origem; v abre o transcript para leitura.
type Sessions struct {
	svc           *session.Service
	home          string
	sessions      []agent.Session
	list          list.Model
	vp            viewport.Model
	detailVP      viewport.Model // conteúdo rolável do painel de detalhe
	paneFocus     kit.PaneID     // painel com foco: lista (padrão) ou detalhe
	detailID      string         // sessão mostrada no detalhe, p/ resetar o scroll ao trocar
	mode          sessMode
	docTitle      string
	docSession    agent.Session // sessão do transcript aberto, p/ exportar com x
	docEntries    []agent.Entry
	agentFilter   string // "" = todas; senão, só sessões desse agente
	grouped       bool   // vista agrupada por agente+projeto; nunca persiste, sempre abre flat
	selected      map[string]bool
	confirm       bool
	dirInput      textinput.Model
	pendingResume agent.Session
	aliasInput    textinput.Model
	aliasTarget   agent.Session // sessão cujo apelido está sendo editado

	// busca full-text nos transcripts: searchIDs != nil = busca
	// ativa, filtra a lista para o subconjunto que bateu; esc restaura.
	searchInput   textinput.Model
	searchIDs     map[string]bool
	searchQuery   string
	toast         string
	toastErr      bool
	toastSeq      int           // auto-dismiss do toast
	spin          spinner.Model // animação de operações lentas
	inFlight      bool
	width, height int

	// usage de tokens/custo por sessão: carregado lazy ao focar,
	// nunca no scan — cacheado por ID pra não refazer o trabalho.
	usageCache map[string]agent.Usage
	usageOK    map[string]bool // sessionID → teve usage encontrado (tried = chave presente)
	usageBusy  map[string]bool // fetch em andamento
}

// sessToastExpire pede para limpar o toast se ele ainda for o de número seq.
type sessToastExpire struct{ seq int }

func sessExpireToastCmd(seq int) tea.Cmd {
	return tea.Tick(kit.ToastTTL, func(time.Time) tea.Msg { return sessToastExpire{seq} })
}

func NewSessions(svc *session.Service, home string) Sessions {
	l := list.New(nil, kit.PlainDelegate{}, 0, 0)
	kit.StyleList(&l)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)  // "N items" fica no título do Panel
	l.SetShowPagination(false) // sem dots crus
	l.DisableQuitKeybindings()
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(theme.Primary)))
	return Sessions{svc: svc, home: home, list: l, vp: viewport.New(), detailVP: viewport.New(), spin: sp}
}

// beginSpin liga o spinner com um rótulo de progresso e devolve o tick inicial.
func (m *Sessions) beginSpin(label string) tea.Cmd {
	m.toast, m.toastErr = label, false
	if m.inFlight {
		return nil
	}
	m.inFlight = true
	return m.spin.Tick
}

func (m Sessions) Init() tea.Cmd { return nil }

// ClearToast some com o toast (usado ao trocar de aba).
func (m *Sessions) ClearToast() { m.toast = "" }

// Count é o total de sessões carregadas (para o contador do header/aba).
func (m Sessions) Count() int { return len(m.sessions) }

func (m Sessions) Capturing() bool {
	return m.mode != sessModeList || m.list.SettingFilter() || m.confirm
}

// Update embrulha update() para agendar o auto-dismiss do toast.
func (m Sessions) step(msg tea.Msg) (Sessions, tea.Cmd) {
	prev := m.toast
	var cmd tea.Cmd
	m, cmd = m.update(msg)
	if m.toast != "" && m.toast != prev {
		m.toastSeq++
		cmd = tea.Batch(cmd, sessExpireToastCmd(m.toastSeq))
	}
	return m, cmd
}

func (m Sessions) update(msg tea.Msg) (Sessions, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.layout()

	case events.AgentsDetected:
		return m, m.loadCmd()

	case sessToastExpire:
		if msg.seq == m.toastSeq && !m.inFlight {
			m.toast = ""
		}
		return m, nil

	case spinner.TickMsg:
		if !m.inFlight {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case events.SessionsLoaded:
		m.inFlight = false
		if msg.Err != nil {
			m.toast, m.toastErr = msg.Err.Error(), true
		}
		m.sessions = msg.Sessions
		return m, m.applyItems()

	case usageMsg:
		delete(m.usageBusy, msg.id)
		if m.usageOK == nil {
			m.usageOK = make(map[string]bool)
			m.usageCache = make(map[string]agent.Usage)
		}
		m.usageOK[msg.id] = msg.ok
		if msg.ok {
			m.usageCache[msg.id] = msg.usage
		}
		if it, sel := m.list.SelectedItem().(sessionItem); sel && it.s.ID == msg.id {
			m.refreshDetail()
		}
		return m, nil

	case searchDoneMsg:
		m.inFlight = false
		if len(msg.matches) == 0 {
			if msg.err != nil {
				m.toast, m.toastErr = msg.err.Error(), true
			} else {
				m.toast, m.toastErr = fmt.Sprintf("nenhuma sessão contém %q", msg.query), true
			}
			return m, nil
		}
		ids := make(map[string]bool, len(msg.matches))
		for _, mt := range msg.matches {
			ids[mt.Session.ID] = true
		}
		m.searchIDs, m.searchQuery = ids, msg.query
		if msg.err != nil {
			m.toast, m.toastErr = fmt.Sprintf("%d sessão(ões) contêm %q (algumas falharam ao ler)", len(msg.matches), msg.query), false
		} else {
			m.toast, m.toastErr = fmt.Sprintf("%d sessão(ões) contêm %q", len(msg.matches), msg.query), false
		}
		m.list.Select(0)
		return m, m.applyItems()

	case aliasDoneMsg:
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
			return m, nil
		}
		for i := range m.sessions { // aplica sem recarregar todos os agentes
			if m.sessions[i].AgentID+":"+m.sessions[i].ID == msg.key {
				m.sessions[i].Alias = msg.alias
			}
		}
		if msg.alias == "" {
			m.toast, m.toastErr = "apelido removido", false
		} else {
			m.toast, m.toastErr = "apelido: "+msg.alias, false
		}
		return m, m.applyItems()

	case resumeDoneMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = "resume terminou com erro: "+msg.err.Error(), true
		} else {
			m.toast, m.toastErr = "de volta ao lazyagents", false
		}
		return m, m.loadCmd()

	case transcriptMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
			return m, nil
		}
		m.docTitle = msg.title
		m.docSession = msg.session
		m.docEntries = msg.entries
		m.vp.SetContent(renderTranscript(msg.entries, m.width-2))
		m.vp.GotoTop()
		m.mode = sessModeDoc
		return m, nil

	case exportDoneMsg:
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
		} else {
			m.toast, m.toastErr = "transcript exportado para "+core.Tilde(msg.path, m.home), false
		}
		return m, nil

	case deleteSessionsMsg:
		m.confirm = false
		m.selected = nil
		if msg.failed > 0 {
			parts := make([]string, len(msg.errs))
			for i, e := range msg.errs {
				parts[i] = e.Error()
			}
			m.toast, m.toastErr = fmt.Sprintf(
				"%d deletada(s), %d falha(s): %s",
				msg.deleted, msg.failed, strings.Join(parts, "; "),
			), true
		} else {
			m.toast, m.toastErr = fmt.Sprintf(
				"%d sessão(ões) movida(s) para o backup", msg.deleted,
			), false
		}
		return m, m.loadCmd()

	case tea.MouseWheelMsg:
		if m.mode == sessModeDoc {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		if m.paneFocus == kit.PaneDetail { // roda rola o detalhe focado
			var cmd tea.Cmd
			m.detailVP, cmd = m.detailVP.Update(msg)
			return m, cmd
		}
		if msg.Button == tea.MouseWheelUp {
			m.list.CursorUp()
		} else if msg.Button == tea.MouseWheelDown {
			m.list.CursorDown()
		}
		return m, m.refreshDetail()

	case tea.MouseClickMsg:
		if m.width < 76 && m.paneFocus == kit.PaneDetail && m.mode == sessModeList {
			return m, nil
		}
		if m.mode == sessModeDoc {
			m.mode = sessModeList // clique fecha a leitura
			return m, nil
		}
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		if msg.X >= m.listWidth() {
			m.paneFocus = kit.PaneDetail // clique no painel de detalhe o foca
			return m, nil
		}
		m.paneFocus = kit.PaneList
		idx := kit.ListIndexAt(&m.list, msg.Y)
		if idx < 0 {
			return m, nil
		}
		if idx == m.list.Index() {
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				svc, s := m.svc, it.s
				spin := m.beginSpin("carregando transcript…")
				return m, tea.Batch(spin, func() tea.Msg {
					entries, err := svc.Transcript(s)
					return transcriptMsg{session: s, title: s.Title, entries: entries, err: err}
				})
			}
			return m, nil
		}
		m.list.Select(idx)
		return m, m.refreshDetail()

	case tea.PasteMsg:
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = kit.FeedTextToList(m.list, msg.Content)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.mode == sessModeDoc {
			switch msg.String() {
			case "esc", "q", "v":
				m.mode = sessModeList
				return m, nil
			case "x":
				svc, sess, entries := m.svc, m.docSession, m.docEntries
				return m, func() tea.Msg {
					path, err := svc.ExportTranscript(sess, entries)
					return exportDoneMsg{path: path, err: err}
				}
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		if m.mode == sessModeSearch {
			return m.updateSearch(msg)
		}
		if m.mode == sessModeDir {
			return m.updateDirPicker(msg)
		}
		if m.mode == sessModeAlias {
			return m.updateAlias(msg)
		}
		if m.confirm {
			switch msg.String() {
			case "enter", "y":
				targets := m.selectedSessions()
				return m, m.deleteCmd(targets)
			case "esc", "n", "q":
				m.confirm = false
			}
			return m, nil
		}
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		// ←/→ movem o foco entre lista e detalhe.
		switch msg.String() {
		case "left":
			m.paneFocus = kit.PaneList
			return m, nil
		case "right":
			m.paneFocus = kit.PaneDetail
			return m, nil
		}
		if m.paneFocus == kit.PaneDetail && kit.DetailScrollKeys[msg.String()] {
			var cmd tea.Cmd
			m.detailVP, cmd = m.detailVP.Update(msg)
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
				spin := m.beginSpin("carregando transcript…")
				return m, tea.Batch(spin, func() tea.Msg {
					entries, err := svc.Transcript(s)
					return transcriptMsg{session: s, title: s.Title, entries: entries, err: err}
				})
			}
		case "R":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				_, dir, ok2 := m.svc.ResumeCmd(it.s)
				if !ok2 {
					m.toast, m.toastErr = it.s.AgentName+" não suporta resume via CLI", true
					return m, nil
				}
				res := m.openDirPicker(it.s, dir)
				return res, nil
			}
			return m, nil
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
		case "space":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				cmd := m.toggleSelect(it.s.ID)
				return m, cmd
			}
			if h, ok := m.list.SelectedItem().(sessionGroupHeader); ok {
				return m, m.toggleSelectGroup(h.ids)
			}
		case "g":
			m.grouped = !m.grouped
			m.list.Select(0)
			return m, m.applyItems()
		case "d":
			sel := m.selectedSessions()
			if len(sel) == 0 {
				if it, ok := m.list.SelectedItem().(sessionItem); ok {
					cmd := m.toggleSelect(it.s.ID)
					m.confirm = true
					return m, cmd
				}
				return m, nil
			}
			m.confirm = true
			return m, nil
		case "r":
			return m, tea.Batch(m.beginSpin("recarregando…"), m.loadCmd())
		case "f":
			m.agentFilter = m.nextAgentFilter()
			m.list.Select(0)
			return m, m.applyItems()
		case "m":
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				return m.openAlias(it.s), nil
			}
			return m, nil
		case "F":
			inp := components.NewInput()
			inp.Placeholder = "buscar nos transcripts…"
			inp.SetWidth(60)
			inp.Focus()
			m.searchInput = inp
			m.mode = sessModeSearch
			return m, nil
		case "esc":
			if m.searchIDs != nil {
				m.searchIDs, m.searchQuery = nil, ""
				m.list.Select(0)
				return m, m.applyItems()
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, tea.Batch(cmd, m.refreshDetail()) // o cursor pode ter mudado
	}
	// mensagens internas dos bubbles (ex.: list.FilterMatchesMsg, que entrega
	// o resultado assíncrono do filtro) precisam chegar à lista
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Sessions) listWidth() int {
	if m.width < 76 {
		return m.width
	}
	return m.width * 3 / 5
}

// bodyHeight é a altura disponível para o corpo (descontados hints + toast).
func (m Sessions) bodyHeight() int {
	h := m.height - 2
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Sessions) layout() tea.Cmd {
	if m.width == 0 {
		return nil
	}
	bodyH := m.bodyHeight()
	lp := components.Panel{Width: m.listWidth(), Height: bodyH}
	m.list.SetSize(lp.ContentWidth(), lp.ContentHeight())
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(bodyH)
	if m.mode == sessModeDoc {
		m.vp.SetContent(renderTranscript(m.docEntries, m.width-2))
	}
	return m.refreshDetail()
}

func (m *Sessions) ID() string { return "sessions" }

func (m *Sessions) Title() string { return "Sessões" }

// Update aplica a mensagem e guarda o novo estado (semântica de ponteiro do
// module.Module). events.Reload equivale à tecla r.
func (m *Sessions) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(events.Reload); ok {
		msg = tea.KeyPressMsg{Code: 'r', Text: "r"}
	}
	nm, cmd := m.step(msg)
	*m = nm
	return cmd
}
