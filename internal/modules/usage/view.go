package usage

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// bar desenha a barra de percentual de uma janela de limite.
func bar(percent float64, width int) string {
	if width < 4 {
		width = 4
	}
	filled := int(percent / 100 * float64(width))
	filled = max(0, min(width, filled))
	style := kit.StOn
	switch {
	case percent >= 90:
		style = kit.StErr
	case percent >= 70:
		style = kit.StWarn
	}
	return style.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(theme.Border).Render(strings.Repeat("░", width-filled))
}

// resetIn formata o tempo restante até o reset da janela.
func resetIn(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Until(t)
	if d <= 0 {
		return "reseta agora"
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf("reseta em %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("reseta em %dh%02dmin", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("reseta %s", t.Local().Format("02/01 15:04"))
	}
}

// limitLines são os limites de um agente em linhas: cabeçalho com o nome na
// cor do agente, autenticação e plano; uma linha por janela (rótulo, barra,
// percentual, reset) e, se a consulta falhou, o erro inteiro, quebrado.
func (m Tab) limitLines(st Status, w int, labelW int) string {
	badge := kit.StOn.Render(st.AuthLabel)
	if st.Auth == agent.AuthAPIKey {
		badge = kit.StWarn.Render(st.AuthLabel)
	} else if st.Auth == agent.AuthUnknown {
		badge = kit.StOff.Render(st.AuthLabel)
	}
	head := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render("● ") + m.agentLabel(st.AgentID) + "  " + badge
	if st.Limits.Plan != "" {
		head += kit.StHint.Render(" · ") + kit.StTitle.Render(st.Limits.Plan)
	}
	if st.Cached && !st.Limits.FetchedAt.IsZero() {
		head += kit.StHint.Render("  (cache de " + st.Limits.FetchedAt.Local().Format("15:04") + ")")
	}
	lines := []string{head}
	const indent = "   "
	if st.Err != "" {
		lines = append(lines, indent+kit.StWarn.Render("! Falha ao atualizar"), kit.Wrap(kit.StText.Render(st.Err), w, indent))
		if len(st.Limits.Windows) > 0 {
			lines = append(lines, indent+kit.StWarn.Render("limites anteriores preservados"))
		}
		lines = append(lines, indent+kit.StHint.Render("r tenta novamente"))
	} else if len(st.Limits.Windows) == 0 {
		lines = append(lines, indent+kit.StHint.Render("sem limites de assinatura para mostrar"))
	}
	for _, win := range st.Limits.Windows {
		pct := fmt.Sprintf("%5.1f%%", win.UsedPercent)
		reset := kit.StHint.Render(resetIn(win.ResetsAt))
		// Linha única: rótulo · barra · % · reset; sem espaço, o rótulo sobe.
		barW := min(28, w-len(indent)-labelW-1-7-2-lipgloss.Width(reset))
		if barW >= 8 && lipgloss.Width(win.Label) <= labelW {
			lines = append(lines, indent+kit.CardLabel.Render(fmt.Sprintf("%-*s", labelW, win.Label))+" "+
				bar(win.UsedPercent, barW)+" "+pct+"  "+reset)
			continue
		}
		lines = append(lines, indent+kit.CardLabel.Render(win.Label),
			indent+bar(win.UsedPercent, max(4, min(28, w-len(indent)-8)))+" "+pct)
		if r := resetIn(win.ResetsAt); r != "" {
			lines = append(lines, indent+kit.StHint.Render(r))
		}
	}
	return strings.Join(lines, "\n")
}

func humanTokens(n int) string { return compact(n) + " tokens" }

// compact abrevia uma contagem: 1.2M, 15.3k, 999.
func compact(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// chip é uma opção de filtro; a ativa fica destacada.
func chip(label string, on bool) string {
	if on {
		return lipgloss.NewStyle().Foreground(theme.Bg).Background(theme.Primary).Bold(true).Padding(0, 1).Render(label)
	}
	return kit.StHint.Padding(0, 1).Render(label)
}

// filterBar mostra os filtros e a tecla de cada um: todas as opções numa
// linha quando cabe, uma linha por filtro quando não, e só as ativas no
// terminal estreito.
func (m Tab) filterBar() string {
	var ps, vs []string
	for _, p := range periods {
		ps = append(ps, p.label)
	}
	for _, v := range tabViews {
		vs = append(vs, v.label)
	}
	ids := append([]string{""}, agentsIn(m.statuses, m.events)...)
	ags := make([]string, len(ids))
	cur := 0
	for i, id := range ids {
		ags[i] = m.name(id)
		if id == m.f.agent {
			cur = i
		}
	}
	text := ""
	if m.f.text != "" && !m.filtering { // digitando, o texto já está no input do rodapé
		text = kit.StHint.Render("/ ") + kit.StTitle.Render(m.f.text) + kit.StHint.Render("  (esc limpa)")
	}

	if m.width < 76 {
		agentName := ansi.Truncate(ags[cur], max(4, m.width-20), "…")
		lines := []string{kit.StHint.Render("p ") + chip(ps[m.f.period], true) + kit.StHint.Render(" a ") + chip(agentName, true), kit.StHint.Render("v ") + chip(vs[m.f.view], true)}
		if text != "" {
			lines = append(lines, text)
		}
		return strings.Join(lines, "\n")
	}

	group := func(key string, opts []string, active int) string {
		parts := []string{components.Keycap(key)}
		for i, o := range opts {
			parts = append(parts, chip(o, i == active))
		}
		return strings.Join(parts, "")
	}
	groups := []string{group("p", ps, m.f.period), group("a", ags, cur), group("v", vs, m.f.view)}
	// Empacota os grupos em linhas: os três numa só quando cabe, senão
	// quantos couberem por linha.
	sep := kit.StHint.Render("  │  ")
	var lines []string
	line := ""
	for _, g := range groups {
		switch {
		case line == "":
			line = g
		case lipgloss.Width(line+sep+g) <= m.width:
			line += sep + g
		default:
			lines, line = append(lines, line), g
		}
	}
	lines = append(lines, line)
	if text != "" {
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

// name é o nome de exibição do agente ("" = todos); sem detecção, o id.
func (m Tab) name(id string) string {
	if id == "" {
		return "todos"
	}
	if n := m.names[id]; n != "" {
		return n
	}
	return id
}

// agentLabel é o nome do agente na cor dele (título e tabela).
func (m Tab) agentLabel(id string) string {
	return lipgloss.NewStyle().Foreground(theme.AgentColor(id)).Bold(true).Render(m.name(id))
}

// body monta a tela inteira (antes do recorte de rolagem).
func (m Tab) body() string {
	if len(m.statuses) == 0 && len(m.events) == 0 {
		if m.loading {
			return kit.StHint.Render("carregando uso…")
		}
		return lipgloss.NewStyle().Width(max(1, m.width)).Render(kit.StHint.Render("Nenhum dado de uso disponível.\nAbra uma sessão em um agente compatível e pressione r para atualizar."))
	}
	now := time.Now()
	var parts []string

	var sts []Status
	api := map[string]bool{}
	for _, st := range m.statuses {
		if st.Auth == agent.AuthAPIKey {
			api[st.AgentID] = true
		}
		if m.f.agent == "" || st.AgentID == m.f.agent {
			sts = append(sts, st)
		}
	}
	if lim := m.limits(sts); lim != "" {
		parts = append(parts, lim)
	}

	events := m.f.apply(m.events, now)
	from := m.f.from(m.events, now)
	price := pricerFor(api)
	showCost := len(api) > 0 && (m.f.agent == "" || api[m.f.agent])
	whole := Sum(events, price)

	var sum strings.Builder
	sum.WriteString(kit.StTitle.Render(upperFirst(periods[m.f.period].label)) + "  ")
	if days := fillDays(Daily(events, 0, nil), from, now); len(days) > 1 {
		keep := max(7, min(len(days), m.width-50)) // a sparkline cabe na linha
		sum.WriteString(sparkline(days[len(days)-min(keep, len(days)):]) + "  ")
	}
	facts := []string{humanTokens(whole.Tokens), fmt.Sprintf("%d respostas", whole.Events)}
	if showCost {
		facts = append(facts, money(whole))
	}
	sum.WriteString(strings.Join(facts, kit.StHint.Render(" · ")))
	if block, ok := Current(Blocks(events, now)); ok {
		left := block.End.Sub(now)
		sum.WriteString("\n" + kit.StTitle.Render("Bloco atual") + kit.StHint.Render(fmt.Sprintf(
			"  desde %s · %dh%02dmin restantes · ", block.Start.Local().Format("15:04"),
			int(left.Hours()), int(left.Minutes())%60)) + humanTokens(Tokens(block.Usage)))
	}
	parts = append(parts, ansi.Wrap(sum.String(), max(1, m.width), ""))

	view := tabViews[m.f.view].id
	title := kit.StTitle.Render(viewTitles[view]) + kit.StHint.Render(" · "+periods[m.f.period].label)
	if m.f.agent != "" {
		title += kit.StHint.Render(" · ") + m.agentLabel(m.f.agent)
	}
	rows := m.f.rows(events, price, from, now)
	switch {
	case len(events) == 0:
		parts = append(parts, title+"\n\n"+kit.StHint.Render("  sem uso registrado neste período"))
	case len(rows) == 0:
		parts = append(parts, title+"\n\n"+kit.StHint.Render(ansi.Wrap(fmt.Sprintf("Nenhuma linha contém %q.\nEsc limpa o filtro.", m.f.text), max(1, m.width), "")))
	default:
		foot := whole
		if m.f.text != "" {
			foot = sumTotals(rows, price != nil)
		}
		cols := colsFull
		switch {
		case m.width < 64:
			cols = colsMinimal
		case m.width < 100:
			cols = colsMedium
		}
		tbl := strings.TrimRight(totalsTable(view, rows, foot, whole.Tokens, showCost, cols, m.agentLabel), "\n")
		if showCost && !foot.Priced {
			tbl += "\n" + kit.StHint.Render("  — custo indisponível: conta por assinatura ou modelo sem preço na tabela")
		}
		parts = append(parts, title+"\n\n"+tbl)
	}
	return strings.Join(parts, "\n\n")
}

// limits é o bloco de limites da assinatura: um agente após o outro, com as
// barras alinhadas entre todos.
func (m Tab) limits(sts []Status) string {
	if len(sts) == 0 {
		return ""
	}
	labelW := 0
	for _, st := range sts {
		for _, win := range st.Limits.Windows {
			labelW = max(labelW, lipgloss.Width(win.Label))
		}
	}
	labelW = min(labelW, 20)
	blocks := []string{kit.StTitle.Render("Limites")}
	for _, st := range sts {
		blocks = append(blocks, m.limitLines(st, m.width, labelW))
	}
	return strings.Join(blocks, "\n")
}

// bodyLines é o corpo recortado à largura útil (cards e colunas nunca
// vazam), em linhas para a rolagem.
func (m Tab) bodyLines() []string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	return strings.Split(clamp.Render(m.body()), "\n")
}

func (m Tab) View() string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	lines := m.lines
	if lines == nil { // View antes de qualquer Update
		lines = m.bodyLines()
		m.bar = lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.filterBar())
	}
	h := m.contentHeight()
	start := min(m.scroll, max(0, len(lines)-h))
	visible := lines[start:min(len(lines), start+h)]
	if rest := len(lines) - start - h; rest > 0 && h > 1 {
		visible = append(visible[:h-1:h-1], kit.StHint.Render(fmt.Sprintf("  ↓ mais %d linhas · j/pgdn", rest+1)))
	}
	return kit.Frame(clamp.Render(strings.Join(visible, "\n")), m.bottom(), m.height)
}

// bottom é o que fica preso embaixo: filtros, atalhos (ou o input do
// filtro de texto) e a linha de progresso ou aviso.
func (m Tab) bottom() string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	foot := kit.Hints(m.width, [2]string{"p", "período"}, [2]string{"a", "agente"}, [2]string{"←→", "visão"},
		[2]string{"/", "filtrar"}, [2]string{"↑↓", "rolar"}, [2]string{"r", "atualizar"}, [2]string{"?", "atalhos"})
	if m.filtering {
		m.input.SetWidth(max(1, m.width-5))
		m.input.SetCursor(m.input.Position())
		foot = kit.StHint.Render("/ ") + m.input.View() + "\n" + kit.StHint.Render("enter mantém · esc limpa")
	}
	bar := lipgloss.NewStyle().MaxHeight(max(1, m.height-4)).Render(m.bar)
	out := []string{bar, clamp.Render(foot)}
	if m.loading {
		out = append(out, kit.StHint.Render("… atualizando uso"))
	} else if m.toast != "" {
		out = append(out, ansi.Truncate(components.Toast(m.toast, m.toastErr), max(1, m.width), "…"))
	}
	return strings.Join(out, "\n")
}

// contentHeight é a altura rolável: o que sobra acima do que fica preso embaixo.
func (m Tab) contentHeight() int {
	return max(1, m.height-lipgloss.Height(m.bottom()))
}
