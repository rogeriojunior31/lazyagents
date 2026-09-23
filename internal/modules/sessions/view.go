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
			components.Keycap("enter")+kit.StHint.Render(" confirma  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"),
		)
		return components.Panel{Title: "Retomar em pasta", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeDoc {
		head := kit.StTitle.Render(kit.Truncate(m.docTitle, 100)) +
			kit.StHint.Render("  transcript · esc volta · x exporta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View(), m.toastLine())
	}
	if m.mode == sessModeAlias {
		w := min(m.width, 72)
		content := lipgloss.JoinVertical(lipgloss.Left,
			kit.StHint.Render(kit.Truncate(m.aliasTarget.Title, w-6)),
			"",
			m.aliasInput.View(),
			"",
			components.Keycap("enter")+kit.StHint.Render(" salva (vazio remove)  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"),
		)
		return components.Panel{Title: "Apelido da sessão", Focused: true, Width: w}.Render(content)
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
			components.Keycap("enter")+kit.StHint.Render(" busca  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"),
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
	filterHint := kit.StHint.Render("f agente")
	if m.agentFilter != "" {
		st := lipgloss.NewStyle().Foreground(theme.AgentColor(m.agentFilter))
		filterHint = kit.StText.Render("f agente: ") + st.Render("● "+tagLabel(m.agentFilter))
	}
	searchHint := kit.StHint.Render("F busca")
	if m.searchIDs != nil {
		searchHint = kit.StText.Render("F busca: ") + kit.StOn.Render(fmt.Sprintf("%q", m.searchQuery)) + kit.StHint.Render(" (esc limpa)")
	}
	var hints string
	if m.confirm {
		n := len(m.selectedSessions())
		hints = kit.StErr.Render(fmt.Sprintf(
			"⚠  deletar %d sessão(ões)? (backup em %s/sessions)  enter confirma · esc cancela",
			n, core.Tilde(m.svc.BackupsDir(), m.home),
		))
	} else {
		groupHint := kit.StHint.Render("g flat")
		if m.grouped {
			groupHint = kit.StText.Render("g agrupada")
		}
		hints = kit.Hints(m.width, [2]string{"enter", "retomar"}, [2]string{"v", "transcript"},
			[2]string{"space", "selecionar"}, [2]string{"f", "agente"}, [2]string{"F", "buscar"},
			[2]string{"/", "filtrar"}, [2]string{"?", "atalhos"})
		if m.agentFilter != "" || m.searchIDs != nil || m.grouped {
			hints = lipgloss.NewStyle().MaxWidth(m.width).Render(groupHint + " · " + filterHint + " · " + searchHint)
		}
		if m.width < 76 {
			hints = kit.Hints(m.width, [2]string{"←/→", "lista / detalhe"}, [2]string{"/", "filtrar"}, [2]string{"?", "atalhos"})
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// --- module.Module ---
