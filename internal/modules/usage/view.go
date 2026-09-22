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

// totals renderiza uma lista rotulada (dias ou projetos) com os tokens.
func totals(title string, items []Total, limit int) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(kit.StTitle.Render(title) + "\n")
	for i, t := range items {
		if i >= limit {
			break
		}
		b.WriteString(fmt.Sprintf("  %-18s %s\n", kit.Truncate(t.Label, 18), kit.CardValue.Render(humanTokens(t.Tokens))))
	}
	return b.String()
}

func humanTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM tokens", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk tokens", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d tokens", n)
	}
}

// body monta a tela inteira (antes do recorte de rolagem).
func (m Tab) body() string {
	if len(m.statuses) == 0 && len(m.events) == 0 {
		if m.loading {
			return kit.StHint.Render("carregando uso…")
		}
		return kit.StHint.Render("nenhum agente com informação de uso. r tenta de novo.")
	}
	cardW := min(m.width, 56)
	var blocks []string
	for _, st := range m.statuses {
		name := st.AgentID
		blocks = append(blocks, components.Panel{
			Title: name, Width: cardW, Border: theme.AgentColor(st.AgentID),
		}.Render(m.statusCard(st, cardW)))
	}
	grid := strings.Join(blocks, "\n")
	if m.width >= 2*cardW+2 && len(blocks) > 1 {
		var rows []string
		for i := 0; i < len(blocks); i += 2 {
			row := blocks[i]
			if i+1 < len(blocks) {
				row = lipgloss.JoinHorizontal(lipgloss.Top, blocks[i], "  ", blocks[i+1])
			}
			rows = append(rows, row)
		}
		grid = strings.Join(rows, "\n")
	}

	var detail strings.Builder
	if block, ok := Current(Blocks(m.events, time.Now())); ok {
		left := time.Until(block.End)
		detail.WriteString(kit.StTitle.Render("Bloco atual") + kit.StHint.Render(
			fmt.Sprintf("  desde %s · %s restantes · %s",
				block.Start.Local().Format("15:04"),
				fmt.Sprintf("%dh%02dmin", int(left.Hours()), int(left.Minutes())%60),
				humanTokens(Tokens(block.Usage)))) + "\n\n")
	}
	daily := totals(fmt.Sprintf("Últimos %d dias", historyDays), Daily(m.events, historyDays), historyDays)
	proj := totals("Por projeto", ByProject(m.events), 5)
	if daily != "" || proj != "" {
		cols := daily
		if m.width >= 70 && daily != "" && proj != "" {
			cols = lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Width(m.width/2).MaxWidth(m.width/2).Render(daily),
				lipgloss.NewStyle().MaxWidth(m.width-m.width/2).Render(proj))
		} else if proj != "" {
			cols = daily + "\n" + proj
		}
		detail.WriteString(cols)
	} else if len(m.events) == 0 {
		detail.WriteString(kit.StHint.Render(fmt.Sprintf("sem transcripts nos últimos %d dias", historyDays)))
	}
	return grid + "\n\n" + strings.TrimRight(detail.String(), "\n")
}

func (m Tab) View() string {
	// cada linha é recortada à largura útil: cards e colunas nunca vazam.
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	lines := strings.Split(clamp.Render(m.body()), "\n")
	h := max(1, m.height-2)
	start := min(m.scroll, max(0, len(lines)-h))
	view := strings.Join(lines[start:min(len(lines), start+h)], "\n")
	hints := clamp.Render(kit.Hints(m.width, [2]string{"r", "atualizar"}, [2]string{"?", "atalhos"}))
	out := []string{view, hints}
	if m.toast != "" {
		out = append(out, clamp.Render(components.Toast(m.toast, m.toastErr)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}
