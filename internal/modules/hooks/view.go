package hooks

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// tagLabel encurta o id do agente para a coluna da matriz.
func tagLabel(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, "-cli"), "-code")
}

// matrix desenha a tabela hook × agente: uma coluna por agente que suporta
// hooks, marcada onde o hook está instalado.
func (m Tab) matrix(width int) string {
	const colW, nameW = 10, 16
	evW := 16
	cmdW := max(10, width-nameW-evW-4-colW*len(m.statuses))

	var b strings.Builder
	b.WriteString(kit.StHint.Render(fmt.Sprintf("  %-*s %-*s %-*s", nameW, "HOOK", evW, "EVENTO", cmdW, "COMANDO")))
	for i, st := range m.statuses {
		label := fmt.Sprintf("%d %s", i+1, kit.Truncate(tagLabel(st.AgentID), colW-3))
		b.WriteString(lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", colW, label)))
	}
	b.WriteString("\n")

	for i, h := range m.lib {
		cursor, name := "  ", fmt.Sprintf("%-*s", nameW, kit.Truncate(h.Name, nameW))
		if i == m.cursor {
			cursor, name = kit.StOn.Render("▸ "), kit.StTitle.Render(name)
		}
		b.WriteString(cursor + name + " " +
			fmt.Sprintf("%-*s ", evW, kit.Truncate(h.Event, evW)) +
			fmt.Sprintf("%-*s", cmdW, kit.Truncate(h.Command, cmdW)))
		for _, st := range m.statuses {
			// Padding à mão: o marcador vem com escapes ANSI, e %-*s contaria
			// os escapes como largura.
			mark := kit.StOff.Render("·")
			switch {
			case enabledIn(st, h.Name):
				mark = kit.StOn.Render("●")
			case !supportsEventName(st, h.Event):
				mark = kit.StHint.Render("–") // o agente não dispara esse evento
			}
			b.WriteString("  " + mark + strings.Repeat(" ", colW-3))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func supportsEventName(st Status, event string) bool {
	for _, e := range st.Events {
		if strings.EqualFold(e, event) {
			return true
		}
	}
	return false
}

// agentLine resume a situação de um agente: arquivo, hooks alheios e o aviso
// do próprio CLI (Codex com hooks desligados, confiança pendente).
func agentLine(st Status, width int) string {
	name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-14s", kit.Truncate(st.AgentName, 14)))
	var detail string
	switch {
	case st.Err != "":
		detail = kit.StErr.Render(st.Err)
	case !st.Installed:
		detail = kit.StHint.Render("não instalado")
	default:
		detail = fmt.Sprintf("%d do lazyagents", len(st.Enabled))
		if st.Foreign > 0 {
			detail += kit.StHint.Render(fmt.Sprintf("  %d próprio(s), intocado(s)", st.Foreign))
		}
	}
	out := "  " + name + " " + detail + "\n" +
		kit.StHint.Render("    "+kit.Truncate(st.File, max(10, width-4)))
	if st.Note != "" {
		out += "\n" + kit.StWarn.Render("    "+kit.Truncate(st.Note, max(10, width-4)))
	}
	return out
}

func (m Tab) body() string {
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("lendo a biblioteca e as configs…")
	}
	if len(m.statuses) == 0 {
		return kit.StHint.Render("nenhum agente instalado suporta hooks.")
	}
	var b strings.Builder
	if len(m.lib) == 0 {
		b.WriteString(kit.StHint.Render("Biblioteca vazia. Crie um hook pela CLI:") + "\n")
		b.WriteString("  lazyagents hooks add doctor --event SessionStart --command \"lazyagents doctor\"\n")
	} else {
		b.WriteString(m.matrix(m.width) + "\n")
		if h, ok := m.current(); ok && h.Description != "" {
			b.WriteString(kit.StHint.Render("  "+kit.Truncate(h.Description, max(10, m.width-2))) + "\n")
		}
	}
	b.WriteString("\n" + kit.StTitle.Render("Agentes") + "\n")
	for _, st := range m.statuses {
		b.WriteString(agentLine(st, m.width) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Tab) View() string {
	if m.confirm != nil {
		return m.confirm.View()
	}
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	out := []string{clamp.Render(m.body())}
	out = append(out, clamp.Render(kit.Hints(m.width,
		[2]string{"1-9", "instalar no agente"},
		[2]string{"space", "em todos"},
		[2]string{"x", "remover"},
		[2]string{"?", "atalhos"},
	)))
	if m.toast != "" {
		out = append(out, clamp.Render(components.Toast(m.toast, m.toastErr)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}
