package views

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/session"
	"lazyskills/internal/tui/components"
	"lazyskills/internal/tui/theme"
)

type sessMode int

const (
	sessModeList   sessMode = iota
	sessModeDoc             // lendo o transcript de uma sessão
	sessModeDir             // input de pasta para o resume
	sessModeSearch          // input de busca full-text nos transcripts (M8.A3)
)

// Sessions é a aba de sessões unificadas de todos os agentes. enter suspende a
// TUI e retoma a sessão no CLI de origem; v abre o transcript para leitura.
type Sessions struct {
	svc           *session.Service
	home          string
	sessions      []agent.Session
	list          list.Model
	vp            viewport.Model
	detailVP      viewport.Model // conteúdo rolável do painel de detalhe (M7.4)
	paneFocus     paneID         // painel com foco: lista (padrão) ou detalhe
	detailID      string         // sessão mostrada no detalhe, p/ resetar o scroll ao trocar
	mode          sessMode
	docTitle      string
	agentFilter   string // "" = todas; senão, só sessões desse agente
	selected      map[string]bool
	confirm       bool
	dirInput      textinput.Model
	pendingResume agent.Session

	// busca full-text nos transcripts (M8.A3): searchIDs != nil = busca
	// ativa, filtra a lista para o subconjunto que bateu; esc restaura.
	searchInput   textinput.Model
	searchIDs     map[string]bool
	searchQuery   string
	toast         string
	toastErr      bool
	toastSeq      int           // auto-dismiss do toast (M7.3)
	spin          spinner.Model // animação de operações lentas (M7.2)
	inFlight      bool
	width, height int

	// usage de tokens/custo por sessão (M8.A1): carregado lazy ao focar,
	// nunca no scan — cacheado por ID pra não refazer o trabalho.
	usageCache map[string]agent.Usage
	usageOK    map[string]bool // sessionID → teve usage encontrado (tried = chave presente)
	usageBusy  map[string]bool // fetch em andamento
}

// sessToastExpire pede para limpar o toast se ele ainda for o de número seq.
type sessToastExpire struct{ seq int }

func sessExpireToastCmd(seq int) tea.Cmd {
	return tea.Tick(toastTTL, func(time.Time) tea.Msg { return sessToastExpire{seq} })
}

type sessionsMsg struct {
	sessions []agent.Session
	err      error
}

type resumeDoneMsg struct{ err error }

type searchDoneMsg struct {
	query   string
	matches []session.Match
	err     error
}

type transcriptMsg struct {
	title   string
	entries []agent.Entry
	err     error
}

type usageMsg struct {
	id    string
	usage agent.Usage
	ok    bool
}

type deleteSessionsMsg struct {
	deleted int
	failed  int
	errs    []error
}

type sessionItem struct {
	s     agent.Session
	title string
	desc  string
}

func (i sessionItem) Title() string       { return i.title }
func (i sessionItem) Description() string { return i.desc }

// FilterValue: agente + título + basename do CWD. O path completo fica fora
// de propósito — o fuzzy caseando letras espalhadas pelos paths tornava o
// filtro inútil (M1.2). Só o basename permite filtrar por projeto.
func (i sessionItem) FilterValue() string {
	v := tagLabel(i.s.AgentID) + " " + i.s.Title
	if base := filepath.Base(i.s.CWD); base != "" && base != "." {
		v += " " + base
	}
	return v
}

var tagStyles = map[string]lipgloss.Style{
	"claude-code": lipgloss.NewStyle().Foreground(theme.Warn),
	"codex":       lipgloss.NewStyle().Foreground(theme.Text),
	"gemini-cli":  lipgloss.NewStyle().Foreground(theme.Primary),
	"opencode":    lipgloss.NewStyle().Foreground(theme.OK),
}

func tagLabel(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, "-cli"), "-code")
}

// agentTag devolve a tag colorida do agente, com largura fixa para os títulos
// da lista ficarem alinhados em coluna.
func agentTag(id string) string {
	label := tagLabel(id)
	pad := strings.Repeat(" ", max(0, 8-len(label)))
	st, ok := tagStyles[id]
	if !ok {
		st = stHint
	}
	return st.Render("⏺ "+label) + pad
}

// relTime formata a idade da sessão de forma humana.
func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("há %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("há %dh", int(d.Hours()))
	case d < 48*time.Hour:
		return "ontem"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("há %dd", int(d.Hours()/24))
	default:
		return t.Format("02/01/2006")
	}
}

// humanCount formata uma contagem de tokens de forma compacta (12345 → 12.3k).
func humanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatUsage resume o consumo de tokens de uma sessão (M8.A1).
func formatUsage(u agent.Usage) string {
	parts := []string{humanCount(u.Input) + " in", humanCount(u.Output) + " out"}
	if cache := u.CacheRead + u.CacheWrite; cache > 0 {
		parts = append(parts, "cache "+humanCount(cache))
	}
	return strings.Join(parts, " · ")
}

func newSessionItem(s agent.Session, home string, live bool) sessionItem {
	cwd := s.CWD
	if cwd == "" {
		cwd = "(pasta desconhecida)"
	}
	title := agentTag(s.AgentID) + " " + s.Title
	if live {
		title = stOn.Render("● ") + title
	}
	return sessionItem{
		s:     s,
		title: title,
		desc:  "  " + relTime(s.MTime) + " · " + tilde(cwd, home),
	}
}

func NewSessions(svc *session.Service, home string) Sessions {
	l := list.New(nil, plainDelegate{}, 0, 0)
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

func (m Sessions) loadCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		sessions, err := svc.List()
		return sessionsMsg{sessions: sessions, err: err}
	}
}

// Update embrulha update() para agendar o auto-dismiss do toast (M7.3).
func (m Sessions) Update(msg tea.Msg) (Sessions, tea.Cmd) {
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

	case AgentsMsg:
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

	case sessionsMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
		}
		m.sessions = msg.sessions
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

	case resumeDoneMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = "resume terminou com erro: "+msg.err.Error(), true
		} else {
			m.toast, m.toastErr = "de volta ao lazyskills", false
		}
		return m, m.loadCmd()

	case transcriptMsg:
		m.inFlight = false
		if msg.err != nil {
			m.toast, m.toastErr = msg.err.Error(), true
			return m, nil
		}
		m.docTitle = msg.title
		m.vp.SetContent(renderTranscript(msg.entries, m.width-2))
		m.vp.GotoTop()
		m.mode = sessModeDoc
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
		if m.paneFocus == paneDetail { // roda rola o detalhe focado (M7.4)
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
		if m.mode == sessModeDoc {
			m.mode = sessModeList // clique fecha a leitura
			return m, nil
		}
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		if msg.X >= m.listWidth() {
			m.paneFocus = paneDetail // clique no painel de detalhe o foca (M7.4)
			return m, nil
		}
		m.paneFocus = paneList
		idx := listIndexAt(&m.list, msg.Y)
		if idx < 0 {
			return m, nil
		}
		if idx == m.list.Index() {
			if it, ok := m.list.SelectedItem().(sessionItem); ok {
				svc, s := m.svc, it.s
				spin := m.beginSpin("carregando transcript…")
				return m, tea.Batch(spin, func() tea.Msg {
					entries, err := svc.Transcript(s)
					return transcriptMsg{title: s.Title, entries: entries, err: err}
				})
			}
			return m, nil
		}
		m.list.Select(idx)
		return m, m.refreshDetail()

	case tea.PasteMsg:
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = feedTextToList(m.list, msg.Content)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.mode == sessModeDoc {
			switch msg.String() {
			case "esc", "q", "v":
				m.mode = sessModeList
				return m, nil
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
		// ←/→ movem o foco entre lista e detalhe (M7.4).
		switch msg.String() {
		case "left":
			m.paneFocus = paneList
			return m, nil
		case "right":
			m.paneFocus = paneDetail
			return m, nil
		}
		if m.paneFocus == paneDetail && detailScrollKeys[msg.String()] {
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
					return transcriptMsg{title: s.Title, entries: entries, err: err}
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
		case "F":
			inp := textinput.New()
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

// resume suspende a TUI e executa o CLI de origem no diretório da sessão.
func (m Sessions) resume(s agent.Session) (Sessions, tea.Cmd) {
	argv, dir, ok := m.svc.ResumeCmd(s)
	if !ok {
		m.toast, m.toastErr = s.AgentName+" não suporta resume via CLI", true
		return m, nil
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		m.toast, m.toastErr = argv[0]+" não está no PATH", true
		return m, nil
	}
	if _, err := os.Stat(dir); err != nil {
		// pasta não existe — abre picker para o usuário corrigir
		return m.openDirPicker(s, dir), nil
	}
	return m, m.runResume(s, dir)
}

func (m Sessions) openDirPicker(s agent.Session, prefill string) Sessions {
	inp := textinput.New()
	inp.SetValue(prefill)
	inp.Focus()
	m.dirInput = inp
	m.pendingResume = s
	m.mode = sessModeDir
	return m
}

func (m Sessions) runResume(s agent.Session, dir string) tea.Cmd {
	argv, _, _ := m.svc.ResumeCmd(s)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return resumeDoneMsg{err: err}
	})
}

func (m Sessions) updateDirPicker(msg tea.KeyPressMsg) (Sessions, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = sessModeList
		return m, nil
	case "enter":
		raw := strings.TrimSpace(m.dirInput.Value())
		if strings.HasPrefix(raw, "~/") {
			raw = filepath.Join(m.home, raw[2:])
		} else if raw == "~" {
			raw = m.home
		}
		if _, err := os.Stat(raw); err != nil {
			m.toast, m.toastErr = "pasta não encontrada: "+raw, true
			return m, nil
		}
		m.mode = sessModeList
		return m, m.runResume(m.pendingResume, raw)
	default:
		var cmd tea.Cmd
		m.dirInput, cmd = m.dirInput.Update(msg)
		return m, cmd
	}
}

// updateSearch trata o input de busca full-text nos transcripts (M8.A3).
func (m Sessions) updateSearch(msg tea.KeyPressMsg) (Sessions, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = sessModeList
		m.searchInput.Blur()
		return m, nil
	case "enter":
		q := strings.TrimSpace(m.searchInput.Value())
		m.searchInput.Blur()
		m.mode = sessModeList
		if q == "" {
			return m, nil
		}
		svc, sessions := m.svc, m.sessions
		spin := m.beginSpin("buscando \"" + q + "\" nos transcripts…")
		return m, tea.Batch(spin, func() tea.Msg {
			matches, err := svc.Search(sessions, q)
			return searchDoneMsg{query: q, matches: matches, err: err}
		})
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// applyItems repõe os itens da lista respeitando o filtro de agente ativo e a
// busca full-text (M8.A3), se houver uma ativa.
func (m *Sessions) applyItems() tea.Cmd {
	items := make([]list.Item, 0, len(m.sessions))
	for _, s := range m.sessions {
		if m.agentFilter != "" && s.AgentID != m.agentFilter {
			continue
		}
		if m.searchIDs != nil && !m.searchIDs[s.ID] {
			continue
		}
		it := newSessionItem(s, m.home, m.svc.IsLive(s))
		if m.selected[s.ID] {
			it.title = "✓ " + it.title
		}
		items = append(items, it)
	}
	cmd := m.list.SetItems(items)
	return tea.Batch(cmd, m.refreshDetail())
}

// toggleSelect alterna a seleção de uma sessão pelo ID.
func (m *Sessions) toggleSelect(id string) tea.Cmd {
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	if m.selected[id] {
		delete(m.selected, id)
	} else {
		m.selected[id] = true
	}
	return m.applyItems()
}

// selectedSessions devolve as sessões marcadas para deleção.
func (m Sessions) selectedSessions() []agent.Session {
	var out []agent.Session
	for _, s := range m.sessions {
		if m.selected[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// deleteCmd executa a deleção das sessões em background e reporta o resultado.
func (m Sessions) deleteCmd(targets []agent.Session) tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		var deleted, failed int
		var errs []error
		for _, s := range targets {
			if err := svc.DeleteSession(s); err != nil {
				failed++
				errs = append(errs, fmt.Errorf("%s: %w", truncate(s.Title, 40), err))
			} else {
				deleted++
			}
		}
		return deleteSessionsMsg{deleted: deleted, failed: failed, errs: errs}
	}
}

// nextAgentFilter cicla todas → cada agente com sessões → todas.
func (m Sessions) nextAgentFilter() string {
	var cycle []string
	seen := map[string]bool{}
	for _, s := range m.sessions {
		if !seen[s.AgentID] {
			seen[s.AgentID] = true
			cycle = append(cycle, s.AgentID)
		}
	}
	if len(cycle) == 0 {
		return ""
	}
	for i, id := range cycle {
		if id == m.agentFilter {
			if i+1 < len(cycle) {
				return cycle[i+1]
			}
			return "" // fim do ciclo: volta para todas
		}
	}
	return cycle[0]
}

func (m Sessions) listWidth() int { return m.width * 3 / 5 }

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
	return m.refreshDetail()
}

// maxChatWidth é a largura de leitura dos cards do transcript — mesmo com o
// terminal largo, a conversa não estica além disso.
const maxChatWidth = 96

// renderTranscript formata as mensagens como cards empilhados (estilo chat):
// um Panel por mensagem, papel no título, cor por papel e uma linha de respiro
// entre os turnos. width é a largura disponível no viewport.
func renderTranscript(entries []agent.Entry, width int) string {
	if len(entries) == 0 {
		return stHint.Render("(transcript vazio ou em formato desconhecido)")
	}
	cardW := min(width, maxChatWidth)
	if cardW < 8 {
		cardW = 8
	}
	cards := make([]string, 0, len(entries))
	for _, e := range entries {
		p := components.Panel{Width: cardW}
		if e.Role == "user" {
			p.Title, p.Border = "▶ você", theme.Primary
		} else {
			p.Title, p.Border = "◀ agente", theme.Subtle
		}
		body := strings.TrimRight(renderMarkdown(e.Text, p.ContentWidth()), " \t\r\n")
		cards = append(cards, p.Render(body), "")
	}
	return strings.Join(cards, "\n")
}

// detailDims devolve largura/altura do painel de detalhe (alinhado à lista).
func (m Sessions) detailDims() (int, int) {
	w := m.width - m.listWidth() - 2 // "  " de gap entre os painéis
	if w < 24 {
		w = 24
	}
	return w, m.bodyHeight()
}

// refreshDetail recomputa o conteúdo do detalhe no viewport, mantendo o scroll
// (volta ao topo só quando a sessão selecionada muda) (M7.4). Devolve o Cmd
// que busca o uso de tokens da sessão em foco (M8.A1), se ainda não tentado.
func (m *Sessions) refreshDetail() tea.Cmd {
	w, h := m.detailDims()
	p := components.Panel{Width: w, Height: h}
	m.detailVP.SetWidth(p.ContentWidth())
	m.detailVP.SetHeight(p.ContentHeight())
	sel, ok := m.list.SelectedItem().(sessionItem)
	id := ""
	if ok {
		id = sel.s.ID
	}
	if id != m.detailID {
		m.detailVP.GotoTop()
		m.detailID = id
	}
	var cmd tea.Cmd
	if ok {
		cmd = m.maybeLoadUsageCmd(sel.s)
	}
	m.detailVP.SetContent(m.detailContent(p.ContentWidth()))
	return cmd
}

// maybeLoadUsageCmd dispara a busca de uso da sessão se ainda não foi tentada
// e não há uma em voo — lazy e cacheado por ID (M8.A1): nunca no
// startup/scan, só ao focar, e nunca refeito pra sessão já respondida.
func (m *Sessions) maybeLoadUsageCmd(s agent.Session) tea.Cmd {
	if _, tried := m.usageOK[s.ID]; tried {
		return nil
	}
	if m.usageBusy[s.ID] {
		return nil
	}
	if m.usageBusy == nil {
		m.usageBusy = make(map[string]bool)
	}
	m.usageBusy[s.ID] = true
	svc := m.svc
	return func() tea.Msg {
		u, ok := svc.SessionUsage(s)
		return usageMsg{id: s.ID, usage: u, ok: ok}
	}
}

// detailView emoldura o viewport do detalhe; a borda acesa segue o foco (M7.4).
func (m Sessions) detailView(w, h int) string {
	return components.Panel{
		Title:   "Sessão",
		Focused: m.paneFocus == paneDetail,
		Width:   w,
		Height:  h,
	}.Render(m.detailVP.View())
}

// detailContent monta o texto do card da sessão selecionada, em inner colunas.
func (m Sessions) detailContent(inner int) string {
	it, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return stHint.Render("Nenhuma sessão encontrada.")
	}
	s := it.s
	label := func(l string) string { return cardLabel.Render(fmt.Sprintf("%-8s", l)) }
	var b strings.Builder
	b.WriteString(stTitle.Render(truncate(s.Title, 200)) + "\n\n")
	st, okTag := tagStyles[s.AgentID]
	if !okTag {
		st = stHint
	}
	b.WriteString(label("agente") + st.Render(s.AgentName) + "\n")
	b.WriteString(label("quando") + cardValue.Render(relTime(s.MTime)) +
		cardLabel.Render("  ("+s.MTime.Format("02/01/2006 15:04")+")") + "\n")
	if m.svc.IsLive(s) {
		b.WriteString(label("status") + stOn.Render("● ativa") + "\n")
	}
	if s.CWD != "" {
		b.WriteString(label("pasta") + cardValue.Render(tilde(s.CWD, m.home)) + "\n")
	}
	b.WriteString(label("id") + cardLabel.Render(s.ID) + "\n")
	if hasUsage, tried := m.usageOK[s.ID]; tried && hasUsage {
		u := m.usageCache[s.ID]
		b.WriteString(label("tokens") + cardValue.Render(formatUsage(u)) + "\n")
		if cost, okCost := agent.EstimateCost(u); okCost {
			b.WriteString(label("custo") + cardValue.Render(fmt.Sprintf("~US$ %.2f", cost)) + "\n")
		}
	}
	if argv, dir, okCmd := m.svc.ResumeCmd(s); okCmd {
		b.WriteString("\n" + cardLabel.Render("retomar  ") + "\n" +
			mdCode.Render(truncate("cd "+tilde(dir, m.home)+" && "+strings.Join(argv, " "), 3*inner)))
	}
	return lipgloss.NewStyle().Width(inner).Render(b.String())
}

func (m Sessions) View() string {
	if m.mode == sessModeDir {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			"Pasta de trabalho para o resume:",
			"",
			m.dirInput.View(),
			"",
			components.Keycap("enter")+stHint.Render(" confirma  ")+components.Keycap("esc")+stHint.Render(" cancela"),
		)
		return components.Panel{Title: "Retomar em pasta", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeDoc {
		head := stTitle.Render(truncate(m.docTitle, 100)) +
			stHint.Render("  transcript · esc volta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	}
	if m.mode == sessModeSearch {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			"Buscar nos transcripts de todos os agentes:",
			"",
			m.searchInput.View(),
			"",
			components.Keycap("enter")+stHint.Render(" busca  ")+components.Keycap("esc")+stHint.Render(" cancela"),
		)
		return components.Panel{Title: "Busca full-text", Focused: true, Width: w}.Render(content)
	}
	detailW, bodyH := m.detailDims()
	listPanel := components.Panel{
		Title:   fmt.Sprintf("Sessões (%d)", len(m.sessions)),
		Focused: m.paneFocus == paneList,
		Width:   m.listWidth(),
		Height:  bodyH,
	}.Render(m.list.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, "  ", m.detailView(detailW, bodyH))
	filterHint := stHint.Render("f agente")
	if m.agentFilter != "" {
		st, ok := tagStyles[m.agentFilter]
		if !ok {
			st = stHint
		}
		filterHint = stText.Render("f agente: ") + st.Render("⏺ "+tagLabel(m.agentFilter))
	}
	searchHint := stHint.Render("F busca")
	if m.searchIDs != nil {
		searchHint = stText.Render("F busca: ") + stOn.Render(fmt.Sprintf("%q", m.searchQuery)) + stHint.Render(" (esc limpa)")
	}
	var hints string
	if m.confirm {
		n := len(m.selectedSessions())
		hints = stErr.Render(fmt.Sprintf(
			"⚠  deletar %d sessão(ões)? (backup em %s/sessions)  enter confirma · esc cancela",
			n, tilde(m.svc.BackupsDir(), m.home),
		))
	} else {
		hints = stHint.Render("enter retoma · v transcript · c cmd · space seleciona · d deleta · / filtra · ") +
			filterHint + stHint.Render(" · ") + searchHint + stHint.Render(" · r recarrega")
	}
	toast := ""
	if m.toast != "" {
		if m.inFlight {
			toast = m.spin.View() + " " + stHint.Render(m.toast)
		} else {
			toast = components.Toast(m.toast, m.toastErr)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, toast)
}
