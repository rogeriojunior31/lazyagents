package sessions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// detailDims devolve largura/altura do painel de detalhe (alinhado à lista).
func (m Tab) detailDims() (int, int) {
	if m.width < 76 {
		return m.width, m.bodyHeight()
	}
	w := m.width - m.listWidth() - 2 // "  " de gap entre os painéis
	if w < 24 {
		w = 24
	}
	return w, m.bodyHeight()
}

// refreshDetail recomputa o conteúdo do detalhe no viewport, mantendo o scroll
// (volta ao topo só quando a sessão selecionada muda). Devolve o Cmd
// que busca o uso de tokens da sessão em foco, se ainda não tentado.
func (m *Tab) refreshDetail() tea.Cmd {
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

// detailView emoldura o viewport do detalhe; a borda acesa segue o foco.
func (m Tab) detailView(w, h int) string {
	return components.Panel{
		Title:   "CONTEXTO",
		Focused: m.paneFocus == kit.PaneDetail,
		Width:   w,
		Height:  h,
	}.Render(m.detailVP.View())
}

// detailContent monta o texto do card da sessão selecionada, em inner colunas.
func (m Tab) detailContent(inner int) string {
	it, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return kit.StHint.Render("Nenhuma sessão encontrada.")
	}
	s := it.s
	label := func(l string) string { return kit.CardLabel.Render(fmt.Sprintf("%-8s", l)) }
	var b strings.Builder
	prose := lipgloss.NewStyle().Width(inner) // texto corrido pode quebrar onde der
	if s.Alias != "" {
		b.WriteString(prose.Render(kit.StTitle.Render(s.Alias)) + "\n" + prose.Render(kit.StHint.Render(kit.Truncate(s.Title, 200))) + "\n\n")
	} else {
		b.WriteString(prose.Render(kit.StTitle.Render(kit.Truncate(s.Title, 200))) + "\n\n")
	}
	st := lipgloss.NewStyle().Foreground(theme.AgentColor(s.AgentID))
	b.WriteString(label("agente") + st.Render(s.AgentName) + "\n")
	b.WriteString(label("quando") + kit.CardValue.Render(relTime(s.MTime)) +
		kit.CardLabel.Render("  ("+s.MTime.Format("02/01/2006 15:04")+")") + "\n")
	if m.svc.IsLive(s) {
		b.WriteString(label("status") + kit.StOn.Render("● ativa") + "\n")
	}
	if s.CWD != "" {
		b.WriteString(label("pasta") + value(kit.CardValue.Render(core.Tilde(s.CWD, m.home)), inner) + "\n")
	}
	b.WriteString(label("id") + value(kit.CardLabel.Render(s.ID), inner) + "\n")
	if hasUsage, tried := m.usageOK[s.ID]; tried && hasUsage {
		u := m.usageCache[s.ID]
		b.WriteString(label("tokens") + kit.CardValue.Render(formatUsage(u)) + "\n")
		if cost, okCost := agent.EstimateCost(u); okCost {
			b.WriteString(label("custo") + kit.CardValue.Render(fmt.Sprintf("~US$ %.2f", cost)) + "\n")
		}
	}
	if argv, dir, okCmd := m.svc.ResumeCmd(s); okCmd {
		// Uma linha por comando e quebra só em espaço: o id e as flags
		// saem inteiros para copiar.
		b.WriteString("\n" + kit.CardLabel.Render("retomar") + kit.StHint.Render("  (enter)") + "\n")
		for _, cmd := range []string{"cd " + core.Tilde(dir, m.home), strings.Join(argv, " ")} {
			b.WriteString(kit.MdCode.Render(wrapWords(cmd, max(8, inner))) + "\n")
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
		meta = append(meta, s.MTime.Format("02/01/2006 15:04"))
	}
	counts := fmt.Sprintf("%d prompts · %d respostas · %d comandos", st.prompts, st.replies, st.tools)
	if st.thoughts > 0 {
		counts += fmt.Sprintf(" · %d raciocínios", st.thoughts)
	}
	meta = append(meta, counts)
	pos := fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)
	if m.vp.TotalLineCount() <= m.vp.VisibleLineCount() {
		pos = "tudo"
	}
	left := strings.Join(meta, kit.StHint.Render(" · "))
	left = ansi.Truncate(left, max(10, w-lipgloss.Width(pos)-2), "…")
	gap := strings.Repeat(" ", max(1, w-lipgloss.Width(left)-lipgloss.Width(pos)))
	metaLine := indent + kit.StHint.Render(left) + gap + kit.StShared.Render(pos)

	tools, thinking := "abre comandos", "abre raciocínio"
	if m.docOpts.tools {
		tools = "resume comandos"
	}
	if m.docOpts.thinking {
		thinking = "recolhe raciocínio"
	}
	hints := kit.Hints(m.width,
		[2]string{"n/N", "prompt"}, [2]string{"t", tools}, [2]string{"r", thinking},
		[2]string{"g/G", "início/fim"}, [2]string{"x", "exporta"}, [2]string{"esc", "volta"})
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
			lipgloss.NewStyle().Width(max(1, w-4)).Render("Pasta de trabalho para o resume:"),
			"",
			components.InputView(m.dirInput, w-4),
			"",
			kit.Hints(w-4, [2]string{"enter", "confirma"}, [2]string{"esc", "volta"}),
		)
		return components.Panel{Title: "Retomar em pasta", Focused: true, Width: w}.Render(content)
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
			kit.StHint.Render("Vazio remove o apelido.")+"\n"+kit.Hints(w-4, [2]string{"enter", "salva"}, [2]string{"esc", "volta"}),
		)
		return components.Panel{Title: "Apelido da sessão", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeSearch {
		w := m.width
		if w > 72 {
			w = 72
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(max(1, w-4)).Render("Buscar nos transcripts de todos os agentes:"),
			"",
			components.InputView(m.searchInput, w-4),
			"",
			kit.Hints(w-4, [2]string{"enter", "busca"}, [2]string{"esc", "volta"}),
		)
		return components.Panel{Title: "Busca full-text", Focused: true, Width: w}.Render(content)
	}
	detailW, bodyH := m.detailDims()
	listPanel := components.Panel{
		Title:   fmt.Sprintf("CONVERSAS   %d", len(m.sessions)),
		Focused: m.paneFocus == kit.PaneList,
		Width:   m.listWidth(),
		Height:  bodyH,
	}.Render(kit.ListView(m.list, "Nenhuma conversa encontrada."))
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, "  ", m.detailView(detailW, bodyH))
	if m.width < 76 {
		body = listPanel
		if m.paneFocus == kit.PaneDetail {
			body = m.detailView(detailW, bodyH)
		}
	}
	hints := kit.Hints(m.width, [2]string{"enter", "retomar"}, [2]string{"v", "transcript"},
		[2]string{"space", "selecionar"}, [2]string{"f", "agente"}, [2]string{"F", "buscar"},
		[2]string{"/", "filtrar"}, [2]string{"?", "atalhos"})

	if m.width < 76 {
		hints = kit.Hints(m.width, [2]string{"←/→", "lista / detalhe"}, [2]string{"/", "filtrar"}, [2]string{"?", "atalhos"})
	}
	parts := []string{body}
	if status := m.filterSummary(); status != "" {
		parts = append(parts, kit.StHint.Render(ansi.Truncate(status, m.width, "…")))
	}
	parts = append(parts, hints, m.toastLine())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// --- module.Module ---

func (m Tab) filterSummary() string {
	var parts []string
	if m.agentFilter != "" {
		parts = append(parts, "agente: "+tagLabel(m.agentFilter))
	}
	if m.searchIDs != nil {
		parts = append(parts, fmt.Sprintf("busca: %q", m.searchQuery))
	}
	if m.grouped {
		parts = append(parts, "agrupada")
	}
	return strings.Join(parts, " · ")
}
