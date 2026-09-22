package sessions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

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
		Focused: m.paneFocus == kit.PaneDetail,
		Width:   w,
		Height:  h,
	}.Render(m.detailVP.View())
}

// detailContent monta o texto do card da sessão selecionada, em inner colunas.
func (m Sessions) detailContent(inner int) string {
	it, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return kit.StHint.Render("Nenhuma sessão encontrada.")
	}
	s := it.s
	label := func(l string) string { return kit.CardLabel.Render(fmt.Sprintf("%-8s", l)) }
	var b strings.Builder
	b.WriteString(kit.StTitle.Render(kit.Truncate(s.Title, 200)) + "\n\n")
	st := lipgloss.NewStyle().Foreground(theme.AgentColor(s.AgentID))
	b.WriteString(label("agente") + st.Render(s.AgentName) + "\n")
	b.WriteString(label("quando") + kit.CardValue.Render(relTime(s.MTime)) +
		kit.CardLabel.Render("  ("+s.MTime.Format("02/01/2006 15:04")+")") + "\n")
	if m.svc.IsLive(s) {
		b.WriteString(label("status") + kit.StOn.Render("● ativa") + "\n")
	}
	if s.CWD != "" {
		b.WriteString(label("pasta") + kit.CardValue.Render(core.Tilde(s.CWD, m.home)) + "\n")
	}
	b.WriteString(label("id") + kit.CardLabel.Render(s.ID) + "\n")
	if hasUsage, tried := m.usageOK[s.ID]; tried && hasUsage {
		u := m.usageCache[s.ID]
		b.WriteString(label("tokens") + kit.CardValue.Render(formatUsage(u)) + "\n")
		if cost, okCost := agent.EstimateCost(u); okCost {
			b.WriteString(label("custo") + kit.CardValue.Render(fmt.Sprintf("~US$ %.2f", cost)) + "\n")
		}
	}
	if argv, dir, okCmd := m.svc.ResumeCmd(s); okCmd {
		b.WriteString("\n" + kit.CardLabel.Render("retomar  ") + "\n" +
			kit.MdCode.Render(kit.Truncate("cd "+core.Tilde(dir, m.home)+" && "+strings.Join(argv, " "), 3*inner)))
	}
	return lipgloss.NewStyle().Width(inner).Render(b.String())
}

// toastLine renderiza o toast atual (ou o spinner de operação em curso),
// vazio quando não há nada a mostrar — usado tanto na lista quanto no doc.
func (m Sessions) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.inFlight {
		return m.spin.View() + " " + kit.StHint.Render(m.toast)
	}
	return components.Toast(m.toast, m.toastErr)
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
			components.Keycap("enter")+kit.StHint.Render(" confirma  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"),
		)
		return components.Panel{Title: "Retomar em pasta", Focused: true, Width: w}.Render(content)
	}
	if m.mode == sessModeDoc {
		head := kit.StTitle.Render(kit.Truncate(m.docTitle, 100)) +
			kit.StHint.Render("  transcript · esc volta · x exporta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View(), m.toastLine())
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
		Title:   fmt.Sprintf("Sessões (%d)", len(m.sessions)),
		Focused: m.paneFocus == kit.PaneList,
		Width:   m.listWidth(),
		Height:  bodyH,
	}.Render(m.list.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, "  ", m.detailView(detailW, bodyH))
	filterHint := kit.StHint.Render("f agente")
	if m.agentFilter != "" {
		st := lipgloss.NewStyle().Foreground(theme.AgentColor(m.agentFilter))
		filterHint = kit.StText.Render("f agente: ") + st.Render("⏺ "+tagLabel(m.agentFilter))
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
		hints = kit.StHint.Render("enter retoma · v transcript · c cmd · space seleciona · d deleta · ") +
			groupHint + kit.StHint.Render(" · / filtra · ") +
			filterHint + kit.StHint.Render(" · ") + searchHint + kit.StHint.Render(" · r recarrega")
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// --- module.Module ---
