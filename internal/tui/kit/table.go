package kit

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Column descreve uma coluna de tabela. Width 0 esconde a coluna; a coluna
// Flex (só a primeira conta) fica com a sobra da largura.
type Column struct {
	Title string
	Width int
	Flex  bool
	Align lipgloss.Position
}

const (
	cellGap   = "  "
	rowPrefix = 2 // "▎ " ou dois espaços
)

// selSGR abre o realce da linha selecionada. É reemitido depois de cada reset
// interno, para que células coloridas (estado por agente, tag) mantenham a
// cor sem furar o fundo (um Render com fundo só cobriria até o primeiro reset).
func selSGR() string {
	return ansi.Style{}.BackgroundColor(theme.Sel).ForegroundColor(theme.Primary).Bold().String()
}

// textSGR é a cor do texto comum; também é reaberta após células coloridas.
func textSGR() string { return ansi.Style{}.ForegroundColor(theme.Text).String() }

// widths resolve a largura de cada coluna, dando à Flex o que sobra.
func widths(width int, cols []Column) []int {
	out := make([]int, len(cols))
	used, flex, shown := 0, -1, 0
	for i, c := range cols {
		if c.Flex && flex < 0 {
			flex = i
			shown++
			continue
		}
		if c.Width > 0 {
			out[i] = c.Width
			used += c.Width
			shown++
		}
	}
	if flex >= 0 {
		gaps := max(0, shown-1) * len(cellGap)
		out[flex] = max(0, width-rowPrefix-used-gaps)
	}
	return out
}

// fit corta e alinha uma célula (pode conter ANSI) em w colunas.
func fit(cell string, w int, align lipgloss.Position) string {
	cell = ansi.Truncate(cell, w, "…")
	return lipgloss.PlaceHorizontal(w, align, cell)
}

func joinCells(width int, cols []Column, cells []string) string {
	ws := widths(width, cols)
	parts := make([]string, 0, len(cols))
	for i, c := range cols {
		// A Flex aparece mesmo com 0 colunas para não desalinhar o cabeçalho.
		if ws[i] == 0 && !c.Flex {
			continue
		}
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts = append(parts, fit(cell, ws[i], c.Align))
	}
	return strings.Join(parts, cellGap)
}

// ColumnAt devolve a coluna sob o x (relativo ao início da linha) numa
// linha de TableRow com essa largura, ou -1 no prefixo, num vão ou além.
func ColumnAt(width int, cols []Column, x int) int {
	ws := widths(width, cols)
	pos := rowPrefix
	for i, c := range cols {
		if ws[i] == 0 && !c.Flex {
			continue
		}
		if x >= pos && x < pos+ws[i] {
			return i
		}
		pos += ws[i] + len(cellGap)
	}
	return -1
}

// TableHeader desenha os títulos das colunas, alinhados com TableRow.
// Títulos já coloridos (nome de agente) mantêm a cor.
func TableHeader(width int, cols []Column) string {
	titles := make([]string, len(cols))
	for i, c := range cols {
		titles[i] = c.Title
	}
	row := ansi.Truncate("  "+joinCells(width, cols, titles), max(0, width), "")
	return paint(row, ansi.Style{}.ForegroundColor(theme.Subtle).String())
}

// TableRow desenha uma linha de tabela com exatamente width colunas. Na linha
// selecionada o fundo de seleção cobre a linha inteira sem apagar a cor das
// células.
func TableRow(width int, selected bool, cols []Column, cells ...string) string {
	width = max(0, width)
	prefix, on := "  ", textSGR()
	if selected {
		prefix, on = "▎ ", selSGR()
	}
	row := ansi.Truncate(prefix+joinCells(width, cols, cells), width, "")
	row += strings.Repeat(" ", max(0, width-lipgloss.Width(row)))
	return paint(row, on)
}

// paint aplica sgr à linha inteira, reabrindo-o depois de cada reset interno.
func paint(row, sgr string) string {
	row = strings.NewReplacer("\x1b[m", "\x1b[m"+sgr, "\x1b[0m", "\x1b[0m"+sgr).Replace(row)
	return sgr + row + "\x1b[m"
}

// AgentLabel é o nome curto do agente usado em tags e cabeçalhos de coluna.
func AgentLabel(id string) string {
	for _, suf := range []string{"-cli", "-code", "-agent"} {
		id = strings.TrimSuffix(id, suf)
	}
	return id
}

// AgentColumns cria uma coluna centralizada por agente, com o nome curto na
// cor do agente. Se os nomes não couberem em room colunas, usa a letra única
// (agent.Short) — a matriz continua legível em terminal estreito.
func AgentColumns(ags []agent.Agent, room int) []Column {
	cols := make([]Column, len(ags))
	total := 0
	for _, ag := range ags {
		total += lipgloss.Width(AgentLabel(ag.ID)) + len(cellGap)
	}
	short := total > room
	for i, ag := range ags {
		label := AgentLabel(ag.ID)
		if short && ag.Short != "" {
			label = ag.Short
		}
		cols[i] = Column{
			Title: lipgloss.NewStyle().Foreground(theme.AgentColor(ag.ID)).Render(label),
			Width: lipgloss.Width(label),
			Align: lipgloss.Center,
		}
	}
	return cols
}

// TableDelegate renderiza itens de bubbles/list em uma linha, via TableRow.
// Itens com Cells() viram linha de tabela; os demais (cabeçalho de grupo)
// mostram o Title() esmaecido. Não usa o realce de runas do filtro do
// delegate padrão, que fatia o texto por runa e corromperia o ANSI das células.
type TableDelegate struct {
	Cols func(width int) []Column
}

func (TableDelegate) Height() int                         { return 1 }
func (TableDelegate) Spacing() int                        { return 0 }
func (TableDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d TableDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if it, ok := item.(interface{ Cells() []string }); ok && d.Cols != nil {
		fmt.Fprint(w, TableRow(width, index == m.Index(), d.Cols(width), it.Cells()...))
		return
	}
	if it, ok := item.(interface{ Title() string }); ok {
		fmt.Fprint(w, StHint.Render(ansi.Truncate("  "+ansi.Strip(it.Title()), max(0, width), "…")))
	}
}

// DetailScroll são as teclas que rolam o detalhe de uma aba de tabela; PgUp/PgDn
// ficam com a lista, que pode ter centenas de linhas.
var DetailScroll = map[string]bool{"shift+up": true, "shift+down": true, "ctrl+u": true, "ctrl+d": true}

// DetailScrollMsg traduz shift+↑/↓ para as teclas de linha do viewport, que
// não conhece o modificador.
func DetailScrollMsg(msg tea.KeyPressMsg) tea.KeyPressMsg {
	switch msg.String() {
	case "shift+up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "shift+down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	return msg
}

// DetailSize é a área de texto do detalhe: o painel lateral desconta o padding
// do Panel; a faixa de baixo, a linha de título e o recuo.
func DetailSize(sp Split) (int, int) {
	if sp.Side {
		p := components.Panel{Width: sp.DetailW, Height: sp.DetailH}
		return p.ContentWidth(), p.ContentHeight()
	}
	return max(1, sp.DetailW-2), max(1, sp.DetailH-1)
}

// DetailView emoldura o detalhe: painel à direita ou faixa sob a tabela, com a
// posição de leitura no título quando o texto não cabe.
func DetailView(sp Split, title string, vp viewport.Model) string {
	if vp.TotalLineCount() > vp.Height() {
		title += fmt.Sprintf(" · %d%% · shift+↑↓", int(vp.ScrollPercent()*100))
	}
	if sp.Side {
		return components.Panel{Title: title, Width: sp.DetailW, Height: sp.DetailH}.Render(vp.View())
	}
	rule := StHint.Render("── " + title + " " + strings.Repeat("─", max(0, sp.DetailW-lipgloss.Width(title)-4)))
	body := lipgloss.NewStyle().PaddingLeft(2).Render(vp.View())
	return Frame(lipgloss.JoinVertical(lipgloss.Left, rule, body), "", sp.DetailH)
}
