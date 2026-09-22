package providers

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/provider"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// tagLabel encurta o id do agente para a coluna da matriz.
func tagLabel(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, "-cli"), "-code")
}

// matrix desenha a tabela perfil × agente: uma coluna por agente que suporta
// troca de provedor, marcada onde o perfil está aplicado.
func (m Providers) matrix(width int) string {
	colW := 10
	nameW := 16
	urlW := max(12, width-nameW-4-colW*len(m.statuses))

	var b strings.Builder
	head := fmt.Sprintf("  %-*s %-*s", nameW, "PERFIL", urlW, "ENDPOINT")
	b.WriteString(kit.StHint.Render(head))
	for i, st := range m.statuses {
		label := fmt.Sprintf("%d %s", i+1, kit.Truncate(tagLabel(st.AgentID), colW-3))
		b.WriteString(lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", colW, label)))
	}
	b.WriteString("\n")

	for i, p := range m.profiles {
		cursor, name := "  ", fmt.Sprintf("%-*s", nameW, kit.Truncate(p.Name, nameW))
		if i == m.cursor {
			cursor, name = kit.StOn.Render("▸ "), kit.StTitle.Render(name)
		}
		detail := p.BaseURL
		if detail == "" {
			detail = "(só modelo " + p.Model + ")"
		}
		b.WriteString(cursor + name + " " + fmt.Sprintf("%-*s", urlW, kit.Truncate(detail, urlW)))
		for _, st := range m.statuses {
			mark := kit.StOff.Render("·")
			if st.Profile == p.Name {
				mark = kit.StOn.Render("●")
			}
			// Padding à mão: o marcador vem com escapes ANSI, e %-*s contaria
			// os escapes como largura.
			left := 2
			b.WriteString(strings.Repeat(" ", left) + mark + strings.Repeat(" ", colW-left-1))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// appliedLine resume o que está valendo num agente agora: uma linha com o
// provedor e outra com o arquivo que Apply/Clear reescrevem.
func appliedLine(st provider.Status, width int) string {
	name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-14s", kit.Truncate(st.AgentName, 14)))
	var detail string
	switch {
	case st.Err != "":
		detail = kit.StErr.Render(st.Err)
	case !st.Installed && st.Active:
		detail = kit.StWarn.Render(st.Applied.BaseURL + " · agente não instalado")
	case st.Active:
		detail = st.Applied.BaseURL
		if detail == "" {
			detail = "modelo " + st.Applied.Model
		}
		if st.Profile != "" {
			detail += kit.StHint.Render("  perfil " + st.Profile)
		} else {
			detail += kit.StHint.Render("  (fora do lazyagents)")
		}
		if st.Applied.HasToken {
			detail += kit.StOn.Render("  token ✓")
		}
		if st.Applied.EnvKey != "" {
			detail += kit.StHint.Render("  token em $" + st.Applied.EnvKey)
		}
	default:
		detail = kit.StHint.Render("padrão do agente")
	}
	return "  " + name + " " + detail + "\n" +
		kit.StHint.Render("    "+kit.Truncate(st.File, max(10, width-4)))
}

func (m Providers) body() string {
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("lendo perfis e configs…")
	}
	if len(m.statuses) == 0 {
		return kit.StHint.Render("nenhum agente instalado suporta troca de provedor.")
	}
	var b strings.Builder
	if len(m.profiles) == 0 {
		b.WriteString(kit.StHint.Render("Nenhum perfil ainda. Crie um pela CLI:") + "\n")
		b.WriteString("  lazyagents provider add trabalho --base-url https://… --token -\n")
	} else {
		b.WriteString(m.matrix(m.width) + "\n")
	}
	b.WriteString("\n" + kit.StTitle.Render("Aplicado agora") + "\n")
	for _, st := range m.statuses {
		b.WriteString(appliedLine(st, m.width) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Providers) View() string {
	clamp := lipgloss.NewStyle().MaxWidth(max(1, m.width))
	if m.confirm != nil {
		return m.confirm.View()
	}
	out := []string{clamp.Render(m.body())}
	out = append(out, clamp.Render(kit.Hints(m.width,
		[2]string{"1-9", "aplicar no agente"},
		[2]string{"space", "em todos"},
		[2]string{"x", "limpar"},
		[2]string{"?", "atalhos"},
	)))
	if m.toast != "" {
		out = append(out, clamp.Render(components.Toast(m.toast, m.toastErr)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}
