package hooks

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// agentState é a situação de uma entrada num agente.
type agentState int

const (
	stateOff         agentState = iota
	stateOn                     // todos os comandos suportados instalados
	statePartial                // parte dos comandos instalada
	stateUnsupported            // o agente não dispara nenhum evento da entrada
	stateMissing                // o CLI não está instalado
)

func stateOf(st Status, h Hook) agentState {
	switch {
	case enabledIn(st, h.Name):
		return stateOn
	case partialIn(st, h.Name):
		return statePartial
	case !supportsAnyEvent(st, h):
		return stateUnsupported
	case !st.Installed:
		return stateMissing
	}
	return stateOff
}

// mark é o marcador do estado e o estilo que o colore.
func (s agentState) mark() (string, lipgloss.Style) {
	switch s {
	case stateOn:
		return "●", kit.StOn
	case statePartial:
		return "◐", kit.StWarn
	case stateUnsupported:
		return "–", kit.StHint
	}
	return "○", kit.StOff
}

func (s agentState) label() string {
	switch s {
	case stateOn:
		return "instalado"
	case statePartial:
		return "parcial"
	case stateUnsupported:
		return "não dispara esses eventos"
	case stateMissing:
		return "CLI não instalado"
	}
	return "desligado"
}

// supportsAnyEvent diz se o agente dispara ao menos um dos eventos da
// entrada — um pacote importado costuma misturar eventos de CLIs diferentes.
func supportsAnyEvent(st Status, h Hook) bool {
	for _, want := range h.Events() {
		for _, e := range st.Events {
			if strings.EqualFold(e, want) {
				return true
			}
		}
	}
	return false
}

// displayCommand encurta o comando importado para a tela: sem o prefixo que
// aponta a raiz do plugin para a cópia e com a raiz como "./" — a pasta já
// aparece em ORIGEM.
func displayCommand(command string) string {
	cmd := stripRootExport(command)
	if cmd == command {
		return command
	}
	return strings.ReplaceAll(cmd, pluginRootSh+"/", "./")
}

// hooksTop é o título mais a linha de cabeçalho da tabela de hooks.
const hooksTop = 2

// split reparte o corpo entre a tabela e o detalhe, como nas outras abas de
// tabela; empilhado, a tabela fica com o que precisa até metade do corpo,
// porque o detalhe de um pacote é longo.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight())
	if !sp.Side {
		listH := min(hooksTop+max(1, len(m.lib))+1, max(hooksTop+1, m.bodyHeight()/2))
		sp.ListH, sp.DetailH = listH, m.bodyHeight()-listH
	}
	return sp
}

func (m Tab) listWidth() int { return m.split().ListW }

// listHeight é a altura da tabela no layout atual.
func (m Tab) listHeight() int { return m.split().ListH }

// bodyHeight é a altura do corpo, descontados hints e toast.
func (m Tab) bodyHeight() int { return max(6, m.height-2) }

// detailHeight é a altura do painel de detalhe no layout atual; escolhendo
// comandos em tela empilhada, ele ocupa o corpo inteiro.
func (m Tab) detailHeight() int {
	sp := m.split()
	if m.cmdMode && !sp.Side {
		return m.bodyHeight()
	}
	return sp.DetailH
}

// View limita tudo à largura da aba: rede de segurança para terminal estreito.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.reader != nil {
		return m.readerView()
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  lendo a biblioteca e as configs…")
	}

	bodyH := m.bodyHeight()
	var body string
	if len(m.lib) == 0 {
		p := components.Panel{Title: "HOOKS", Focused: true, Width: m.width, Height: bodyH}
		message := "Nenhum hook instalado.\n\nAbra Skills pela paleta (: skills) e pressione i para importar um repositório com hooks."
		body = p.Render(lipgloss.NewStyle().Width(p.ContentWidth()).Render(message))
	} else if sp := m.split(); !sp.Side && m.cmdMode {
		body = m.detailPanel(m.width, bodyH)
	} else if !sp.Side {
		body = lipgloss.JoinVertical(lipgloss.Left,
			m.tableView(sp.ListW, sp.ListH),
			m.detailPanel(m.width, m.detailHeight()))
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.tableView(sp.ListW, sp.ListH), "  ",
			m.detailPanel(sp.DetailW, bodyH))
	}

	hints := kit.Hints(m.width,
		[2]string{"space", "instala/remove"},
		[2]string{"←→", "agente"},
		[2]string{"enter", "comandos"},
		[2]string{"v", "script"},
		[2]string{"a", "todos"},
		[2]string{"x", "remove de todos"},
		[2]string{"?", "atalhos"})
	if len(m.lib) == 0 {
		hints = kit.Hints(m.width, [2]string{":", "comandos"}, [2]string{"?", "atalhos"})
	} else if m.cmdMode {
		hints = kit.Hints(m.width,
			[2]string{"v", "script"},
			[2]string{"space", "marcar"},
			[2]string{"↑↓", "escolher"},
			[2]string{"pgup/pgdn", "ler"},
			[2]string{"esc", "voltar"})
	}
	out := []string{body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// Colunas da tabela de hooks; os agentes vêm a partir de colAgents.
const (
	colName = iota
	colCount
	colEvents
	colAgents
)

// statusAgents converte os agentes da aba no formato das colunas de agente.
func (m Tab) statusAgents() []agent.Agent {
	ags := make([]agent.Agent, len(m.statuses))
	for i, st := range m.statuses {
		ags[i] = agent.Agent{ID: st.AgentID, Name: st.AgentName, Short: st.Short}
	}
	return ags
}

// tableCols: hook, comandos ligados, eventos (flex) e um agente por coluna,
// com a coluna do cursor sublinhada.
func (m Tab) tableCols(width int) []kit.Column {
	agents := kit.AgentColumns(m.statusAgents(), width/3)
	for i, st := range m.statuses {
		if i == m.col {
			agents[i].Title = lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Underline(true).Bold(true).
				Render(ansi.Strip(agents[i].Title))
		}
	}
	nameW := 4
	for _, h := range m.lib {
		nameW = max(nameW, lipgloss.Width(h.Name))
	}
	cols := []kit.Column{
		{Title: "hook", Width: min(nameW, 24)},
		{Title: "cmds", Width: 5, Align: lipgloss.Right},
		{Title: "eventos", Flex: true},
	}
	return append(cols, agents...)
}

// cells é a linha de um hook: comandos ligados/total, eventos e o estado em
// cada agente. Na linha selecionada a célula sob o cursor aparece invertida.
func (m Tab) cells(h Hook, selected bool) []string {
	count := fmt.Sprintf("%d", len(h.Hooks))
	if len(h.Off) > 0 {
		count = fmt.Sprintf("%d/%d", len(h.Active()), len(h.Hooks))
	}
	out := []string{h.Name, kit.StHint.Render(count), kit.StHint.Render(strings.Join(h.Events(), ", "))}
	for i, st := range m.statuses {
		mark, style := stateOf(st, h).mark()
		cell := style.Render(mark)
		if selected && i == m.col {
			cell = lipgloss.NewStyle().Reverse(true).Render(mark)
		}
		out = append(out, cell)
	}
	return out
}

// legend explica os marcadores da matriz.
var legend = kit.StOn.Render("●") + kit.StHint.Render(" instalado  ") +
	kit.StWarn.Render("◐") + kit.StHint.Render(" parcial  ") +
	kit.StOff.Render("○") + kit.StHint.Render(" desligado  ") +
	kit.StHint.Render("– sem esses eventos")

// tableView é a biblioteca como matriz hook × agente.
func (m Tab) tableView(w, h int) string {
	title := "  " + kit.StTitle.Render("BIBLIOTECA") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.lib)))
	// A legenda vai no título se couber; senão no pé da tabela, se sobrar linha.
	foot := ""
	if gap := w - lipgloss.Width(title) - lipgloss.Width(legend); gap >= 2 {
		title += strings.Repeat(" ", gap) + legend
	} else if h-hooksTop-len(m.lib) >= 2 {
		foot = "  " + legend
	}
	cols := m.tableCols(w)
	lines := []string{title, kit.TableHeader(w, cols)}
	start, end := kit.Window(m.cursor, len(m.lib), max(1, h-hooksTop))
	for i := start; i < end; i++ {
		lines = append(lines, kit.TableRow(w, i == m.cursor, cols, m.cells(m.lib[i], i == m.cursor)...))
	}
	return kit.Frame(strings.Join(lines, "\n"), foot, h)
}

// detailWidth é a largura do painel de detalhe no layout atual.
func (m Tab) detailWidth() int { return m.split().DetailW }

// maxDetailOff é a rolagem máxima do detalhe: a última tela cheia.
func (m Tab) maxDetailOff() int {
	p := components.Panel{Width: m.detailWidth()}
	return max(0, strings.Count(m.detailContent(p.ContentWidth()), "\n")+1-m.detailRows())
}

// detailPanel é o card da entrada selecionada, rolável com pgup/pgdn.
func (m Tab) detailPanel(w, h int) string {
	p := components.Panel{Title: "SOBRE O HOOK", Focused: m.cmdMode, Width: w, Height: h}
	if m.cmdMode {
		if hook, ok := m.current(); ok {
			p.Title = fmt.Sprintf("COMANDOS · %d/%d", m.cmdCursor+1, len(hook.Hooks))
		}
	}
	lines := strings.Split(m.detailContent(p.ContentWidth()), "\n")
	visible := m.detailRows()
	off := min(m.detailOff, max(0, len(lines)-visible))
	lines = lines[off:]
	if len(lines) > visible && visible > 1 {
		lines = append(lines[:visible-1], kit.StHint.Render(fmt.Sprintf("↓ mais %d linhas · pgdn", len(lines)-visible+1)))
	}
	content := strings.Join(lines, "\n")
	if m.cmdMode {
		hook, _ := m.current()
		order := commandOrder(hook)
		start, end := kit.Window(m.cmdCursor, len(order), m.commandRows())
		var choices []string
		for i := start; i < end; i++ {
			idx := order[i]
			mark := "[x] "
			if hook.IsOff(idx) {
				mark = "[ ] "
			}
			prefix, style := "  ", kit.StText
			if i == m.cmdCursor {
				prefix = cmdCursorMark + " "
				style = style.Background(theme.Sel).Bold(true)
			}
			label := ansi.Truncate(prefix+mark+commandLabel(hook.Hooks[idx]), p.ContentWidth(), "…")
			choices = append(choices, style.Width(p.ContentWidth()).Render(label))
		}
		content = strings.Join(choices, "\n") + "\n\n" + content
	}
	return p.Render(content)
}

func (m Tab) commandRows() int {
	hook, _ := m.current()
	return min(len(hook.Hooks), 3, max(1, (m.detailHeight()-2)/3))
}

func (m Tab) detailRows() int {
	h := m.detailHeight() - 2
	if m.cmdMode {
		h -= m.commandRows() + 1
	}
	return max(1, h)
}

func (m Tab) detailContent(inner int) string {
	h, ok := m.current()
	if !ok {
		return strings.Join([]string{
			kit.StText.Render("Nenhum hook na biblioteca."),
			"",
			kit.StHint.Render("Crie um pela CLI:"),
			kit.StText.Render(`  lazyagents hooks add doctor --event SessionStart --command "lazyagents doctor"`),
			"",
			kit.StHint.Render("Ou instale na aba Skills (") + components.Keycap("i") +
				kit.StHint.Render(") um repositório de plugin: o hooks/hooks.json dele vem junto."),
		}, "\n")
	}
	if m.cmdMode {
		order := commandOrder(h)
		if m.cmdCursor >= len(order) {
			return ""
		}
		i := order[m.cmdCursor]
		c := h.Hooks[i]
		state := "ligado"
		if h.IsOff(i) {
			state = "desligado"
		}
		info := c.Event + " · " + state
		if c.Matcher != "" && c.Matcher != "*" {
			info += "\nFiltro: " + c.Matcher
		}
		if flags := commandFlags(c); flags != "" {
			info += " · " + flags
		}
		separator := "\n"
		if inner >= 40 {
			separator = "\n\n"
		}
		return ansi.Wrap(kit.StHint.Render(info)+separator+kit.StText.Render(c.Command), max(1, inner), "")
	}
	home := m.svc.home
	var b strings.Builder
	b.WriteString(kit.StTitle.Render(h.Name) + "\n")
	if h.Description != "" {
		b.WriteString(lipgloss.NewStyle().Width(inner).Render(kit.StText.Render(h.Description)) + "\n")
	}
	b.WriteString("\n" + kit.StHint.Render("NOS AGENTES") + "\n")
	nameW := 0
	for _, st := range m.statuses {
		nameW = max(nameW, lipgloss.Width(st.AgentName))
	}
	for i, st := range m.statuses {
		s := stateOf(st, h)
		mark, style := s.mark()
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", nameW, st.AgentName))
		b.WriteString(fmt.Sprintf("%s %s %s  %s\n", components.Keycap(fmt.Sprintf("%d", i+1)),
			style.Render(mark), name, style.Render(s.label())))
		indent := strings.Repeat(" ", 6)
		var info []string
		if st.Err != "" {
			b.WriteString(kit.Wrap(kit.StErr.Render(st.Err), inner, indent) + "\n")
		}
		info = append(info, core.Tilde(st.File, home))
		if st.Foreign > 0 {
			info = append(info, fmt.Sprintf("%d hook(s) próprio(s), intocado(s)", st.Foreign))
		}
		b.WriteString(kit.Wrap(kit.StHint.Render(strings.Join(info, " · ")), inner, indent) + "\n")
		if st.Note != "" {
			b.WriteString(kit.Wrap(kit.StWarn.Render(st.Note), inner, indent) + "\n")
		}
	}
	if h.Imported() || h.Files != "" {
		b.WriteString("\n" + kit.StHint.Render("ORIGEM") + "\n")
		if h.Source != "" {
			b.WriteString(cardField("origem", h.Source, inner))
		}
		if h.Files != "" {
			b.WriteString(cardField("raiz", core.Tilde(h.Files, home), inner))
		}
		if h.Imported() {
			b.WriteString(kit.Wrap(kit.StWarn.Render("! escrito para o Claude Code; em outro CLI o payload pode mudar"), inner, "") + "\n")
		}
	}
	if p := CommandProblem(h); p != "" {
		b.WriteString(kit.Wrap(kit.StErr.Render("⚠ "+p), inner, "") + "\n")
	}
	title := fmt.Sprintf("COMANDOS   %d", len(h.Hooks))
	if len(h.Off) > 0 {
		title = fmt.Sprintf("COMANDOS   %d de %d ligados", len(h.Active()), len(h.Hooks))
	}
	b.WriteString("\n" + kit.StHint.Render(title) + "\n")
	if !m.cmdMode {
		b.WriteString(kit.StHint.Render("enter escolhe quais comandos instalar") + "\n")
	}
	b.WriteString(commandsBlock(h, inner))

	return strings.TrimRight(b.String(), "\n")
}

// cmdCursorMark identifica o comando selecionado.
const cmdCursorMark = "▸"

// commandOrder é a ordem em que os comandos aparecem no detalhe: agrupados
// por evento. Devolve índices em h.Hooks.
func commandOrder(h Hook) []int {
	var out []int
	for _, ev := range h.Events() {
		for i, c := range h.Hooks {
			if c.Event == ev {
				out = append(out, i)
			}
		}
	}
	return out
}

// commandLabel é o nome curto de um comando: o script que ele roda, ou o
// próprio comando encurtado.
func commandLabel(c agent.Hook) string {
	fields := strings.Fields(displayCommand(c.Command))
	for i := len(fields) - 1; i >= 0; i-- {
		f := strings.Trim(fields[i], `"'`)
		if strings.Contains(f, "/") {
			return filepath.Base(f)
		}
	}
	return kit.Truncate(displayCommand(c.Command), 40)
}

// commandsBlock mantém o resumo compacto; Enter abre a leitura completa.
func commandsBlock(h Hook, inner int) string {
	var lines []string
	for _, ev := range h.Events() {
		lines = append(lines, kit.StShared.Render(ev))
		for i, c := range h.Hooks {
			if c.Event != ev {
				continue
			}
			mark := "[x] "
			if h.IsOff(i) {
				mark = "[ ] "
			}
			lines = append(lines, kit.StText.Render(ansi.Truncate(mark+commandLabel(c), max(1, inner), "…")))
		}
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func commandFlags(h agent.Hook) string {
	var f []string
	if h.Async {
		f = append(f, "async")
	}
	if h.Timeout > 0 {
		f = append(f, fmt.Sprintf("%ds", h.Timeout))
	}
	return strings.Join(f, " · ")
}

// cardField é "rótulo  valor" com o valor quebrado alinhado à própria coluna.
func cardField(label, value string, inner int) string {
	const col = 9
	pad := strings.Repeat(" ", col)
	wrapped := kit.Wrap(kit.CardValue.Render(value), inner, pad)
	return kit.CardLabel.Render(fmt.Sprintf("%-*s", col, label)) + strings.TrimPrefix(wrapped, pad) + "\n"
}
