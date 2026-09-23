package usage

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Texto da CLI de uso. Os estilos são os da TUI; lipgloss.Fprint rebaixa as
// cores ao terminal e as remove quando a saída não é um terminal (pipe,
// arquivo, NO_COLOR).

const cliBarW = 24 // barra de limite e de participação

var viewTitles = map[string]string{
	"daily": "Tokens por dia", "agents": "Tokens por agente",
	"projects": "Tokens por projeto", "models": "Tokens por modelo",
}

var viewHeads = map[string]string{
	"daily": "DIA", "agents": "AGENTE", "projects": "PROJETO", "models": "MODELO",
}

// renderLimits lista as janelas de limite de cada agente.
func renderLimits(sts []Status) string {
	var b strings.Builder
	for i, st := range sts {
		if i > 0 {
			b.WriteString("\n")
		}
		meta := []string{st.AuthLabel}
		if st.Limits.Plan != "" {
			meta = append([]string{st.Limits.Plan}, meta...)
		}
		if st.Cached && !st.Limits.FetchedAt.IsZero() {
			meta = append(meta, "cache de "+st.Limits.FetchedAt.Local().Format("15:04"))
		}
		b.WriteString(agentName(st.AgentID) + "  " + kit.StHint.Render(strings.Join(meta, " · ")) + "\n")
		switch {
		case st.Err != "" && len(st.Limits.Windows) == 0:
			b.WriteString("  " + kit.StErr.Render("✗ "+st.Err) + "\n")
			continue
		case st.Err != "":
			b.WriteString("  " + kit.StWarn.Render("! "+st.Err+" — mostrando o cache") + "\n")
		case len(st.Limits.Windows) == 0:
			b.WriteString("  " + kit.StHint.Render("sem limites de assinatura para mostrar") + "\n")
		}
		for _, w := range st.Limits.Windows {
			fmt.Fprintf(&b, "  %-16s %s %5.1f%%  %s\n", kit.Truncate(w.Label, 16),
				bar(w.UsedPercent, cliBarW), w.UsedPercent, kit.StHint.Render(resetIn(w.ResetsAt)))
		}
	}
	return b.String()
}

// renderTotals é a tabela de uma visão agregada (dia, agente, projeto, modelo).
func renderTotals(o usageOpts, rows []Total, hidden int, total Total, showCost bool) string {
	var b strings.Builder
	b.WriteString(kit.StTitle.Render(viewTitles[o.view]) + kit.StHint.Render(" · "+o.period) + "\n\n")
	b.WriteString(totalsTable(o.view, rows, total, total.Tokens, showCost, colsFull))
	if hidden > 0 {
		b.WriteString(kit.StHint.Render(fmt.Sprintf("  … mais %d (--limit 0 mostra todas)", hidden)) + "\n")
	}
	if showCost && !total.Priced {
		b.WriteString(kit.StHint.Render("  — custo indisponível: conta por assinatura ou modelo sem preço na tabela") + "\n")
	}
	return b.String()
}

// Quantas colunas cabem: a TUI encolhe a tabela com a largura da aba.
const (
	colsFull    = iota // tokens, entrada, saída, cache, respostas, custo, participação
	colsMedium         // tokens, respostas, custo, participação
	colsMinimal        // tokens, participação
)

// totalsTable é a tabela de uma visão, com foot como linha de total e a
// participação de cada linha relativa a whole tokens. Compartilhada entre a
// CLI e a aba (que filtra linhas: o rodapé soma as visíveis, a participação
// continua sobre o período inteiro).
func totalsTable(view string, rows []Total, foot Total, whole int, showCost bool, cols int) string {
	head := []string{viewHeads[view], "TOKENS"}
	if cols == colsFull {
		head = append(head, "ENTRADA", "SAÍDA", "CACHE")
	}
	if cols <= colsMedium {
		head = append(head, "RESPOSTAS")
		if showCost {
			head = append(head, "CUSTO")
		}
	}
	head = append(head, "")
	barW, labelW := cliBarW, 28
	if cols == colsMinimal {
		barW, labelW = 10, 14
	}
	cells := func(t Total, label string) []string {
		r := []string{label, compact(t.Tokens)}
		if cols == colsFull {
			r = append(r, compact(t.Usage.Input), compact(t.Usage.Output), compact(t.Usage.CacheRead+t.Usage.CacheWrite))
		}
		if cols <= colsMedium {
			r = append(r, fmt.Sprint(t.Events))
			if showCost {
				r = append(r, money(t))
			}
		}
		return r
	}
	var body [][]string
	for _, t := range rows {
		label := t.Label
		switch view {
		case "agents":
			label = agentName(t.Label)
		case "daily":
			label = dayLabel(t.Label)
		default:
			label = kit.Truncate(label, labelW)
		}
		body = append(body, append(cells(t, label), share(t.Tokens, whole, barW)))
	}
	return table(head, body, append(cells(foot, "total"), ""))
}

// renderPanel é o `usage` sem visão: limites, bloco atual e o resumo do período.
func renderPanel(o usageOpts, sts []Status, events []agent.UsageEvent, price Pricer, showCost bool, now time.Time) string {
	var parts []string
	if len(sts) > 0 {
		parts = append(parts, strings.TrimRight(renderLimits(sts), "\n"))
	}
	if block, ok := Current(Blocks(events, now)); ok {
		left := block.End.Sub(now)
		parts = append(parts, kit.StTitle.Render("Bloco atual")+"  "+kit.StHint.Render(fmt.Sprintf(
			"desde %s · %dh%02dmin restantes · ", block.Start.Local().Format("15:04"),
			int(left.Hours()), int(left.Minutes())%60))+humanTokens(Tokens(block.Usage)))
	}

	total := Sum(events, price)
	var b strings.Builder
	days := fillDays(Daily(events, 0, price), o.from, now)
	summary := []string{humanTokens(total.Tokens), fmt.Sprintf("%d respostas", total.Events)}
	if showCost {
		summary = append(summary, money(total))
	}
	b.WriteString(kit.StTitle.Render(upperFirst(o.period)) + "  ")
	if len(days) > 1 {
		b.WriteString(sparkline(days) + "  ")
	}
	b.WriteString(strings.Join(summary, kit.StHint.Render(" · ")))
	if len(events) == 0 {
		b.WriteString("\n  " + kit.StHint.Render("sem uso registrado nos transcripts"))
	}
	if agents := ByAgent(events, price); len(agents) > 0 {
		var rows [][]string
		for _, t := range agents {
			rows = append(rows, []string{agentName(t.Label), compact(t.Tokens), share(t.Tokens, total.Tokens, cliBarW)})
		}
		b.WriteString("\n" + strings.TrimRight(table(nil, rows, nil), "\n"))
	}
	if projs := ByProject(events, price); len(projs) > 0 {
		b.WriteString("\n\n" + kit.StTitle.Render("Top projetos") + "\n")
		var rows [][]string
		for _, t := range projs[:min(3, len(projs))] {
			rows = append(rows, []string{kit.Truncate(t.Label, 28), compact(t.Tokens), share(t.Tokens, total.Tokens, cliBarW)})
		}
		b.WriteString(strings.TrimRight(table(nil, rows, nil), "\n"))
	}
	parts = append(parts, b.String())
	parts = append(parts, kit.StHint.Render("detalhe: lazyagents usage limits|daily|agents|projects|models · help usage"))
	return strings.Join(parts, "\n\n") + "\n"
}

// table alinha colunas pela largura visível (estilos ANSI não contam): a
// primeira à esquerda, as outras à direita. head e foot são opcionais; foot
// vem depois de um traço.
func table(head []string, rows [][]string, foot []string) string {
	all := append([][]string{head}, rows...)
	all = append(all, foot)
	var w []int
	for _, r := range all {
		for i, cell := range r {
			if i == len(w) {
				w = append(w, 0)
			}
			w[i] = max(w[i], lipgloss.Width(cell))
		}
	}
	line := func(r []string) string {
		parts := make([]string, len(r))
		for i, cell := range r {
			pad := strings.Repeat(" ", w[i]-lipgloss.Width(cell))
			if i == 0 {
				parts[i] = cell + pad
			} else {
				parts[i] = pad + cell
			}
		}
		return "  " + strings.TrimRight(strings.Join(parts, "  "), " ")
	}
	var b strings.Builder
	if head != nil {
		b.WriteString(kit.StHint.Render(line(head)) + "\n")
	}
	for _, r := range rows {
		b.WriteString(line(r) + "\n")
	}
	if foot != nil {
		width := len(w) * 2
		for _, x := range w {
			width += x
		}
		b.WriteString(kit.StHint.Render("  "+strings.Repeat("─", width-2)) + "\n")
		b.WriteString(kit.StTitle.Render(line(foot)) + "\n")
	}
	return b.String()
}

// share é a barra de participação de part no total, com o percentual.
func share(part, total, width int) string {
	frac := 0.0
	if total > 0 {
		frac = float64(part) / float64(total)
	}
	filled := int(frac*float64(width) + 0.5)
	if part > 0 && filled == 0 {
		filled = 1 // uso pequeno ainda aparece
	}
	return kit.StShared.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(theme.Border).Render(strings.Repeat("░", width-filled)) +
		fmt.Sprintf(" %3.0f%%", frac*100)
}

// sparkline desenha uma série de dias, relativa ao maior.
func sparkline(days []Total) string {
	const ticks = "▁▂▃▄▅▆▇█"
	peak := 0
	for _, d := range days {
		peak = max(peak, d.Tokens)
	}
	var b strings.Builder
	for _, d := range days {
		if d.Tokens == 0 || peak == 0 {
			b.WriteString(kit.StOff.Render("·"))
			continue
		}
		i := min(7, d.Tokens*8/(peak+1))
		b.WriteString(kit.StShared.Render(string([]rune(ticks)[i])))
	}
	return b.String()
}

func agentName(id string) string {
	return lipgloss.NewStyle().Foreground(theme.AgentColor(id)).Bold(true).Render(id)
}

// dayLabel mostra o dia com a semana: "ter 23/09"; hoje fica destacado.
func dayLabel(day string) string {
	if day == time.Now().Format("2006-01-02") {
		return kit.StTitle.Render(dayText(day))
	}
	return dayText(day)
}

// dayText é o rótulo do dia sem estilo (também é o que o filtro de texto casa).
func dayText(day string) string {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return day
	}
	week := [...]string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}[t.Weekday()]
	return week + " " + t.Format("02/01")
}

func money(t Total) string {
	if !t.Priced {
		return "—"
	}
	return fmt.Sprintf("$%.2f", t.Cost)
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
