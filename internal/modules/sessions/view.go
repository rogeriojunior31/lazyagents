package sessions

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// split reparte o corpo entre a tabela e o detalhe; empilhado, a altura
// que a tabela não usa vai para o detalhe.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight())
	if !sp.Side {
		need := len(m.tableHead(sp.ListW)) + max(1, len(m.list.VisibleItems())) + 1
		if spare := sp.ListH - need; spare > 0 {
			sp.ListH -= spare
			sp.DetailH += spare
		}
	}
	return sp
}

// refreshDetail recomputa o conteúdo do detalhe no viewport, mantendo o scroll
// (volta ao topo só quando a sessão selecionada muda). Devolve o Cmd
// que busca o uso de tokens da sessão em foco, se ainda não tentado.
func (m *Tab) refreshDetail() tea.Cmd {
	w, h := kit.DetailSize(m.split())
	m.detailVP.SetWidth(w)
	m.detailVP.SetHeight(h)
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
	m.detailVP.SetContent(m.detailContent(w))
	return cmd
}

// maybeLoadUsageCmd dispara a busca de uso da sessão se ainda não foi tentada
// e não há uma em voo — lazy e cacheado por ID: nunca no
// startup/scan, só ao focar, e nunca refeito pra sessão já respondida.
func (m *Tab) maybeLoadUsageCmd(s agent.Session) tea.Cmd {
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

// detailContent monta o texto da sessão selecionada, em inner colunas: o
// comando de retomar logo abaixo do título, que é o que se vem buscar aqui.
func (m Tab) detailContent(inner int) string {
	if h, ok := m.list.SelectedItem().(sessionGroupHeader); ok {
		return kit.StHint.Render(fmt.Sprintf("%d session(s) in this group — ", len(h.ids))) +
			components.Keycap("space") + kit.StHint.Render(" selects all")
	}
	it, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return kit.StHint.Render("No sessions found.")
	}
	s := it.s
	label := func(l string) string { return kit.CardLabel.Render(fmt.Sprintf("%-8s", l)) }
	var b strings.Builder
	prose := lipgloss.NewStyle().Width(inner) // texto corrido pode quebrar onde der
	if s.Alias != "" {
		b.WriteString(prose.Render(kit.StTitle.Render(s.Alias)) + "\n" + prose.Render(kit.StHint.Render(kit.Truncate(s.Title, 200))) + "\n")
	} else {
		b.WriteString(prose.Render(kit.StTitle.Render(kit.Truncate(s.Title, 200))) + "\n")
	}
	if argv, dir, okCmd := m.svc.ResumeCmd(s); okCmd {
		// Uma linha por comando e quebra só em espaço: o id e as flags
		// saem inteiros para copiar.
		b.WriteString(kit.CardLabel.Render("resume") + kit.StHint.Render("  enter · R in another folder") + "\n")
		for _, cmd := range []string{"cd " + core.Tilde(dir, m.home), strings.Join(argv, " ")} {
			b.WriteString(kit.MdCode.Render(wrapWords(cmd, max(8, inner))) + "\n")
		}
		if s.CWD != "" && dir != s.CWD {
			b.WriteString(prose.Render(kit.StWarn.Render("the session folder no longer exists; resumes in the folder above")) + "\n")
		}
	}
	st := lipgloss.NewStyle().Foreground(theme.AgentColor(s.AgentID))
	when := relTime(s.MTime) + kit.CardLabel.Render("  ("+s.MTime.Format("2006-01-02 15:04")+")")
	if m.svc.IsLive(s) {
		when += "  " + kit.StOn.Render("● open now")
	}
	b.WriteString("\n" + label("agent") + st.Render(s.AgentName) + "\n")
	b.WriteString(label("when") + kit.CardValue.Render(when) + "\n")
	if s.CWD != "" {
		b.WriteString(label("folder") + value(kit.CardValue.Render(core.Tilde(s.CWD, m.home)), inner) + "\n")
	}
	b.WriteString(label("id") + value(kit.CardLabel.Render(s.ID), inner) + "\n")
	if hasUsage, tried := m.usageOK[s.ID]; tried && hasUsage {
		u := m.usageCache[s.ID]
		b.WriteString(label("tokens") + kit.CardValue.Render(formatUsage(u)) + "\n")
		if cost, okCost := agent.EstimateCost(u); okCost {
			b.WriteString(label("cost") + kit.CardValue.Render(fmt.Sprintf("~$%.2f", cost)) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// wrapWords quebra só em espaço, para um argumento (o id da sessão) nunca
// se partir; palavra maior que a linha fica inteira na dela.
func wrapWords(s string, width int) string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) <= width:
			line += " " + w
		default:
			lines, line = append(lines, line), w
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// value quebra o valor de um campo só em espaço (caminho e id não se partem
// no hífen), recuado à coluna dos valores.
func value(v string, inner int) string {
	const col = 8
	lines := strings.Split(ansi.Wrap(v, max(8, inner-col), ""), "\n")
	return strings.Join(lines, "\n"+strings.Repeat(" ", col))
}

// readerView é o leitor de transcript: título, uma linha de contexto com a
// posição da leitura à direita, a conversa e os atalhos.
func (m Tab) readerView() string {
	s, st := m.docSession, m.docView.stats
	// Cabeçalho na mesma coluna centralizada da conversa.
	w, pad := chatColumn(m.width - 2)
	indent := strings.Repeat(" ", pad)
	title := indent + kit.StTitle.Render(ansi.Truncate(m.docTitle, w, "…"))

	meta := []string{lipgloss.NewStyle().Foreground(theme.AgentColor(s.AgentID)).Render(s.AgentName)}
	if s.CWD != "" {
		meta = append(meta, core.Tilde(s.CWD, m.home))
	}
	if !s.MTime.IsZero() {
		meta = append(meta, s.MTime.Format("2006-01-02 15:04"))
	}
	counts := fmt.Sprintf("%d prompts · %d replies · %d commands", st.prompts, st.replies, st.tools)
	if st.thoughts > 0 {
		counts += fmt.Sprintf(" · %d thoughts", st.thoughts)
	}
	meta = append(meta, counts)
	pos := fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)
	if m.vp.TotalLineCount() <= m.vp.VisibleLineCount() {
		pos = "all"
	}
	left := strings.Join(meta, kit.StHint.Render(" · "))
	left = ansi.Truncate(left, max(10, w-lipgloss.Width(pos)-2), "…")
	gap := strings.Repeat(" ", max(1, w-lipgloss.Width(left)-lipgloss.Width(pos)))
	metaLine := indent + kit.StHint.Render(left) + gap + kit.StShared.Render(pos)

	tools, thinking := "expand commands", "expand reasoning"
	if m.docOpts.tools {
		tools = "collapse commands"
	}
	if m.docOpts.thinking {
		thinking = "collapse reasoning"
	}
	hints := kit.Hints(m.width,
		[2]string{"n/N", "prompt"}, [2]string{"t", tools}, [2]string{"r", thinking},
		[2]string{"g/G", "top/end"}, [2]string{"x", "export"}, [2]string{"esc", "back"})
	return lipgloss.JoinVertical(lipgloss.Left, title, metaLine, m.vp.View(), hints, m.toastLine())
}

// toastLine renderiza o toast atual (ou o spinner de operação em curso),
// vazio quando não há nada a mostrar — usado tanto na lista quanto no doc.
func (m Tab) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.inFlight {
		return m.spin.View() + " " + kit.StHint.Render(m.toast)
	}
	return components.Toast(m.toast, m.toastErr)
}

func (m Tab) View() string {
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.mode == sessModeDir {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(max(1, w-4)).Render("Working directory to resume in:"),
			"",
			components.InputView(m.dirInput, w-4),
			"",
			kit.Hints(w-4, [2]string{"enter", "confirm"}, [2]string{"esc", "back"}),
		)
		return components.Panel{Title: "Resume in folder", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeDoc {
		return m.readerView()
	}
	if m.mode == sessModeAlias {
		w := min(m.width, 72)
		content := lipgloss.JoinVertical(lipgloss.Left,
			kit.StHint.Render(kit.Truncate(m.aliasTarget.Title, w-6)),
			"",
			components.InputView(m.aliasInput, w-4),
			"",
			kit.StHint.Render("Leave empty to remove the alias.")+"\n"+kit.Hints(w-4, [2]string{"enter", "save"}, [2]string{"esc", "back"}),
		)
		return components.Panel{Title: "Session alias", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeSearch {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(max(1, w-4)).Render("Search the transcripts of every agent:"),
			"",
			components.InputView(m.searchInput, w-4),
			"",
			kit.Hints(w-4, [2]string{"enter", "search"}, [2]string{"esc", "back"}),
		)
		return components.Panel{Title: "Full-text search", Focused: true, Width: w}.Render(content)
	}
	sp := m.split()
	table := m.tableView(sp.ListW, sp.ListH)
	detail := kit.DetailView(sp, "CONTEXT", m.detailVP)
	body := lipgloss.JoinVertical(lipgloss.Left, table, detail)
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", detail)
	}
	hints := kit.Hints(m.width, [2]string{"enter", "resume"}, [2]string{"v", "transcript"},
		[2]string{"space", "select"}, [2]string{"f", "agent"}, [2]string{"g", "group"},
		[2]string{"F", "search"}, [2]string{"/", "filter"}, [2]string{"?", "help"})
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// tableHead são as linhas acima das conversas: título com os filtros ativos,
// input do filtro (se houver) e os nomes das colunas.
func (m Tab) tableHead(w int) []string {
	title := kit.StTitle.Render("SESSIONS") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.list.VisibleItems())))
	if len(m.list.VisibleItems()) != len(m.sessions) {
		title += kit.StHint.Render(fmt.Sprintf(" of %d", len(m.sessions)))
	}
	if n := len(m.selectedSessions()); n > 0 {
		title += kit.StShared.Render(fmt.Sprintf(" · %d selected", n))
	}
	lines := []string{"  " + title}
	// Filtros ativos ficam à vista: no título se couberem, senão numa linha.
	if f := m.filterSummary(); f != "" {
		if with := lines[0] + kit.StHint.Render(" · ") + kit.StLocal.Render(f); lipgloss.Width(with) <= w {
			lines[0] = with
		} else {
			lines = append(lines, "  "+kit.StLocal.Render(f))
		}
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(0, w), "…")
	}
	if m.list.FilterState() != list.Unfiltered {
		m.list.FilterInput.SetWidth(max(1, w-6))
		lines = append(lines, "  "+m.list.FilterInput.View())
	}
	return append(lines, kit.TableHeader(w, m.tableCols(w)))
}

// tableWindow devolve a faixa de linhas visíveis e onde a primeira é
// desenhada — a mesma conta para renderizar e para o clique.
func (m Tab) tableWindow(w, h int) (start, end, top int) {
	top = len(m.tableHead(w))
	start, end = kit.Window(m.list.Index(), len(m.list.VisibleItems()), max(1, h-top))
	return start, end, top
}

// tableView desenha a tabela de conversas em w×h: uma linha por sessão e,
// na vista agrupada, uma linha por grupo.
func (m Tab) tableView(w, h int) string {
	lines := m.tableHead(w)
	items := m.list.VisibleItems()
	if len(items) == 0 {
		empty := "No sessions found."
		if m.list.FilterState() == list.FilterApplied {
			empty = fmt.Sprintf("Nothing found for “%s”.", m.list.FilterValue())
		}
		lines = append(lines, kit.StHint.Render("  "+empty))
	}
	cols := m.tableCols(w)
	group := []kit.Column{{Flex: true}}
	start, end, _ := m.tableWindow(w, h)
	for i := start; i < end; i++ {
		sel := i == m.list.Index()
		switch it := items[i].(type) {
		case sessionItem:
			lines = append(lines, kit.TableRow(w, sel, cols, m.cells(it, w)...))
		case sessionGroupHeader:
			lines = append(lines, kit.TableRow(w, sel, group, kit.StHint.Render(it.label)))
		}
	}
	return kit.Frame(strings.Join(lines, "\n"), "", h)
}

// --- module.Module ---

func (m Tab) filterSummary() string {
	var parts []string
	if m.agentFilter != "" {
		parts = append(parts, fmt.Sprintf("agent: %s", kit.AgentLabel(m.agentFilter)))
	}
	if m.searchIDs != nil {
		parts = append(parts, fmt.Sprintf("search: %q", m.searchQuery))
	}
	if m.grouped {
		parts = append(parts, "grouped")
	}
	return strings.Join(parts, " · ")
}
