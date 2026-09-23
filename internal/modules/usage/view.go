package usage

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

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

// statusCard monta o card de um agente: autenticação, plano e as janelas.
func (m Tab) statusCard(st Status, w int) string {
	inner := components.Panel{Width: w}.ContentWidth()
	var b strings.Builder
	badge := kit.StOn.Render("● " + st.AuthLabel)
	if st.Auth == agent.AuthAPIKey {
		badge = kit.StWarn.Render("● " + st.AuthLabel)
	} else if st.Auth == agent.AuthUnknown {
		badge = kit.StOff.Render("● " + st.AuthLabel)
	}
	b.WriteString(badge)
	if st.Limits.Plan != "" {
		b.WriteString("  " + kit.StTitle.Render(st.Limits.Plan))
	}
	if st.Cached && !st.Limits.FetchedAt.IsZero() {
		b.WriteString(kit.StHint.Render("  (cache de " + st.Limits.FetchedAt.Local().Format("15:04") + ")"))
	}
	b.WriteString("\n\n")
	if st.Err != "" && len(st.Limits.Windows) == 0 {
		b.WriteString(kit.StHint.Render(kit.Truncate(st.Err, inner)))
		return lipgloss.NewStyle().Width(inner).Render(b.String())
	}
	if len(st.Limits.Windows) == 0 {
		b.WriteString(kit.StHint.Render("sem limites de assinatura para mostrar"))
		return lipgloss.NewStyle().Width(inner).Render(b.String())
	}
	barW := max(8, min(28, inner-34))
	for _, win := range st.Limits.Windows {
		b.WriteString(fmt.Sprintf("%-16s %s %5.1f%%\n",
			kit.Truncate(win.Label, 16), bar(win.UsedPercent, barW), win.UsedPercent))
		if r := resetIn(win.ResetsAt); r != "" {
			b.WriteString(kit.StHint.Render("                 "+r) + "\n")
		}
	}
	return lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n"))
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

// filterBar mostra os filtros e a tecla de cada um. Estreita, vira uma linha.
func (m Tab) filterBar() string {
	agent := "todos"
	if m.f.agent != "" {
		agent = m.f.agent
	}
	if m.width < 76 {
		line := kit.StHint.Render("p ") + chip(periods[m.f.period].label, true) +
			kit.StHint.Render("  a ") + chip(agent, true) + kit.StHint.Render("  v ") + chip(tabViews[m.f.view].label, true)
		if m.f.text != "" {
			line += kit.StHint.Render("  / ") + kit.StTitle.Render(m.f.text)
		}
		return line
	}
	row := func(key, title string, opts []string, active int) string {
		parts := []string{kit.CardLabel.Render(fmt.Sprintf("%-8s", title)), kit.StHint.Render(key)}
		for i, o := range opts {
			parts = append(parts, chip(o, i == active))
		}
		return strings.Join(parts, " ")
	}
	var ps, vs []string
	for _, p := range periods {
		ps = append(ps, p.label)
	}
	for _, v := range tabViews {
		vs = append(vs, v.label)
	}
	ags := append([]string{"todos"}, agentsIn(m.statuses, m.events)...)
	cur := 0
	for i, a := range ags {
		if a == agent {
			cur = i
		}
	}
	lines := []string{
		row("p", "período", ps, m.f.period),
		row("a", "agente", ags, cur),
		row("v", "visão", vs, m.f.view),
	}
	if m.f.text != "" {
		lines[2] += kit.StHint.Render("    / ") + kit.StTitle.Render(m.f.text) + kit.StHint.Render("  (esc limpa)")
	}
	return strings.Join(lines, "\n")
}

// body monta a tela inteira (antes do recorte de rolagem).
func (m Tab) body() string {
	if len(m.statuses) == 0 && len(m.events) == 0 {
		if m.loading {
			return kit.StHint.Render("carregando uso…")
		}
		return kit.StHint.Render("nenhum agente com informação de uso. r tenta de novo.")
	}
	now := time.Now()
	parts := []string{m.filterBar()}

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
	if grid := m.cards(sts); grid != "" {
		parts = append(parts, grid)
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
	parts = append(parts, sum.String())

	view := tabViews[m.f.view].id
	title := kit.StTitle.Render(viewTitles[view]) + kit.StHint.Render(" · "+periods[m.f.period].label)
	if m.f.agent != "" {
		title += kit.StHint.Render(" · ") + agentName(m.f.agent)
	}
	rows := m.f.rows(events, price, from, now)
	switch {
	case len(events) == 0:
		parts = append(parts, title+"\n\n"+kit.StHint.Render("  sem uso registrado neste período"))
	case len(rows) == 0:
		parts = append(parts, title+"\n\n"+kit.StHint.Render(fmt.Sprintf("  nenhuma linha contém %q", m.f.text)))
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
		tbl := strings.TrimRight(totalsTable(view, rows, foot, whole.Tokens, showCost, cols), "\n")
		if showCost && !foot.Priced {
			tbl += "\n" + kit.StHint.Render("  — custo indisponível: conta por assinatura ou modelo sem preço na tabela")
		}
		parts = append(parts, title+"\n\n"+tbl)
	}
	return strings.Join(parts, "\n\n")
}

// cards são os limites da assinatura, dois por linha quando cabe.
func (m Tab) cards(sts []Status) string {
	cardW := min(m.width, 56)
	var blocks []string
	for _, st := range sts {
		blocks = append(blocks, components.Panel{
			Title: st.AgentID, Width: cardW, Border: theme.AgentColor(st.AgentID),
		}.Render(m.statusCard(st, cardW)))
	}
	if m.width < 2*cardW+2 || len(blocks) < 2 {
		return strings.Join(blocks, "\n")
	}
	var rows []string
	for i := 0; i < len(blocks); i += 2 {
		row := blocks[i]
		if i+1 < len(blocks) {
			row = lipgloss.JoinHorizontal(lipgloss.Top, blocks[i], "  ", blocks[i+1])
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
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
	}
	h := max(1, m.height-2)
	start := min(m.scroll, max(0, len(lines)-h))
	view := strings.Join(lines[start:min(len(lines), start+h)], "\n")
	foot := kit.Hints(m.width, [2]string{"p", "período"}, [2]string{"a", "agente"}, [2]string{"←→", "visão"},
		[2]string{"/", "filtrar"}, [2]string{"r", "atualizar"}, [2]string{"?", "atalhos"})
	if m.filtering {
		foot = kit.StHint.Render("/ ") + m.input.View() + kit.StHint.Render("  enter mantém · esc limpa")
	}
	out := []string{view, clamp.Render(foot)}
	if m.toast != "" {
		out = append(out, clamp.Render(components.Toast(m.toast, m.toastErr)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}
