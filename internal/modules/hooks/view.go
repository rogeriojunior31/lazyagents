package hooks

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Mesmo realce de linha do kit.PlainDelegate: a lista daqui é desenhada à
// mão (o cursor é um índice simples), mas precisa parecer a das outras abas.
var (
	selTitle = lipgloss.NewStyle().Background(theme.Sel).Foreground(theme.Primary).Bold(true)
	selDesc  = lipgloss.NewStyle().Background(theme.Sel).Foreground(theme.Subtle)
)

// narrowWidth é a largura abaixo da qual lista e detalhe empilham.
const narrowWidth = 76

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

func (m Tab) listWidth() int {
	if m.width < narrowWidth {
		return m.width
	}
	return max(30, m.width*2/5)
}

// bodyHeight é a altura do corpo, descontados hints e toast.
func (m Tab) bodyHeight() int { return max(6, m.height-2) }

func (m Tab) View() string {
	if m.confirm != nil {
		return m.confirm.View()
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  lendo a biblioteca e as configs…")
	}

	bodyH := m.bodyHeight()
	var body string
	if m.width < narrowWidth {
		listH := min(bodyH/2, 3*len(m.lib)+3)
		body = lipgloss.JoinVertical(lipgloss.Left,
			m.listPanel(m.width, max(4, listH)),
			m.detailPanel(m.width, max(4, bodyH-listH)))
	} else {
		listW := m.listWidth()
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.listPanel(listW, bodyH), "  ",
			m.detailPanel(m.detailWidth(), bodyH))
	}

	hints := kit.Hints(m.width,
		[2]string{"1-9", "liga/desliga no agente"},
		[2]string{"space", "em todos"},
		[2]string{"x", "remove de todos"},
		[2]string{"pgup/pgdn", "rola detalhe"},
		[2]string{"?", "atalhos"})
	out := []string{body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// listPanel é a biblioteca: nome, estado por agente e um resumo por entrada.
func (m Tab) listPanel(w, h int) string {
	p := components.Panel{Title: fmt.Sprintf("BIBLIOTECA   %d", len(m.lib)), Focused: true, Width: w, Height: h}
	inner := p.ContentWidth()
	if len(m.lib) == 0 {
		return p.Render("\n" + kit.StHint.Render("Biblioteca vazia."))
	}

	// Cada entrada ocupa 3 linhas (título, resumo, espaço); a janela segue o
	// cursor quando não cabe tudo.
	per := max(1, (p.ContentHeight()-1)/3)
	start, end := kit.Window(m.cursor, len(m.lib), per)
	lines := []string{""}
	for i := start; i < end; i++ {
		h := m.lib[i]
		marks, plain := m.agentMarks(h)
		nameW := max(4, inner-2-lipgloss.Width(plain)-1)
		name := ansi.Truncate(h.Name, nameW, "…")
		gap := strings.Repeat(" ", max(1, inner-2-lipgloss.Width(name)-lipgloss.Width(plain)))
		summary := ansi.Truncate(entrySummary(h), inner-2, "…")
		if i == m.cursor {
			lines = append(lines,
				selTitle.Width(inner).Render("▎ "+name+gap+plain),
				selDesc.Width(inner).Render("▎ "+summary))
		} else {
			lines = append(lines, "  "+kit.StText.Render(name)+gap+marks, "  "+kit.StHint.Render(summary))
		}
		lines = append(lines, "")
	}
	if end < len(m.lib) {
		lines[len(lines)-1] = kit.StHint.Render(fmt.Sprintf("  ↓ mais %d", len(m.lib)-end))
	}
	return p.Render(strings.Join(lines, "\n"))
}

// agentMarks devolve um marcador por agente, colorido e sem cor.
func (m Tab) agentMarks(h Hook) (colored, plain string) {
	var c, p []string
	for _, st := range m.statuses {
		mark, style := stateOf(st, h).mark()
		c = append(c, style.Render(mark))
		p = append(p, mark)
	}
	return strings.Join(c, " "), strings.Join(p, " ")
}

// entrySummary é a segunda linha da lista: o comando quando é um só, o
// tamanho do pacote quando são vários.
func entrySummary(h Hook) string {
	if len(h.Hooks) == 1 {
		return h.Hooks[0].Event + " · " + displayCommand(h.Hooks[0].Command)
	}
	return fmt.Sprintf("%d comandos · %s", len(h.Hooks), strings.Join(h.Events(), ", "))
}

// detailWidth é a largura do painel de detalhe no layout atual.
func (m Tab) detailWidth() int {
	if m.width < narrowWidth {
		return m.width
	}
	return max(24, m.width-m.listWidth()-2)
}

// maxDetailOff é a rolagem máxima do detalhe: a última tela cheia.
func (m Tab) maxDetailOff() int {
	p := components.Panel{Width: m.detailWidth()}
	return max(0, strings.Count(m.detailContent(p.ContentWidth()), "\n")+1-m.bodyHeight()/2)
}

// detailPanel é o card da entrada selecionada, rolável com pgup/pgdn.
func (m Tab) detailPanel(w, h int) string {
	p := components.Panel{Title: "SOBRE O HOOK", Width: w, Height: h}
	lines := strings.Split(m.detailContent(p.ContentWidth()), "\n")
	visible := p.ContentHeight()
	off := min(m.detailOff, max(0, len(lines)-visible))
	lines = lines[off:]
	if len(lines) > visible && visible > 1 {
		lines = append(lines[:visible-1], kit.StHint.Render(fmt.Sprintf("↓ mais %d linhas · pgdn", len(lines)-visible+1)))
	}
	return p.Render(strings.Join(lines, "\n"))
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
			b.WriteString(indent + kit.StErr.Render(ansi.Truncate(st.Err, inner-6, "…")) + "\n")
		}
		info = append(info, core.Tilde(st.File, home))
		if st.Foreign > 0 {
			info = append(info, fmt.Sprintf("%d hook(s) próprio(s), intocado(s)", st.Foreign))
		}
		b.WriteString(indent + kit.StHint.Render(ansi.Truncate(strings.Join(info, " · "), inner-6, "…")) + "\n")
		if st.Note != "" {
			note := lipgloss.NewStyle().Width(max(10, inner-6)).Render(st.Note)
			for _, ln := range strings.Split(note, "\n") {
				b.WriteString(indent + kit.StWarn.Render(ln) + "\n")
			}
		}
	}
	if h.Imported() || h.Files != "" {
		b.WriteString("\n" + kit.StHint.Render("ORIGEM") + "\n")
		if h.Source != "" {
			b.WriteString(kit.CardLabel.Render("origem   ") + kit.CardValue.Render(h.Source) + "\n")
		}
		if h.Files != "" {
			b.WriteString(kit.CardLabel.Render("raiz     ") + kit.CardValue.Render(core.Tilde(h.Files, home)) + "\n")
		}
		if h.Imported() {
			b.WriteString(kit.StWarn.Render("! escrito para o Claude Code; em outro CLI o payload pode mudar") + "\n")
		}
	}
	if p := CommandProblem(h); p != "" {
		b.WriteString(kit.StErr.Render("⚠ "+p) + "\n")
	}
	b.WriteString("\n" + kit.StHint.Render(fmt.Sprintf("COMANDOS   %d", len(h.Hooks))) + "\n")
	b.WriteString(commandsBlock(h, inner))

	return strings.TrimRight(b.String(), "\n")
}

// commandsBlock lista os comandos agrupados por evento: matcher numa coluna,
// comando na outra e os modificadores (async, timeout) no fim.
func commandsBlock(h Hook, inner int) string {
	matchW := 0
	for _, c := range h.Hooks {
		matchW = max(matchW, len([]rune(matcherLabel(c))))
	}
	matchW = min(matchW, 16)
	var b strings.Builder
	for i, ev := range h.Events() {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(kit.StShared.Render(ev) + "\n")
		for _, c := range h.Hooks {
			if c.Event != ev {
				continue
			}
			flags := commandFlags(c)
			cmdW := max(8, inner-2-matchW-2-lipgloss.Width(flags)-1) // -1: espaço antes das flags
			line := "  " + kit.CardLabel.Render(fmt.Sprintf("%-*s", matchW, kit.Truncate(matcherLabel(c), matchW))) + "  " +
				kit.CardValue.Render(ansi.Truncate(displayCommand(c.Command), cmdW, "…"))
			if flags != "" {
				pad := inner - lipgloss.Width(line) - lipgloss.Width(flags)
				line += strings.Repeat(" ", max(1, pad)) + kit.StHint.Render(flags)
			}
			b.WriteString(line + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func matcherLabel(h agent.Hook) string {
	if h.Matcher == "" || h.Matcher == "*" {
		return "todos"
	}
	return h.Matcher
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
