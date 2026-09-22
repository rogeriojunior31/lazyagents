package kit

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

var (
	mdH1     = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	mdH2     = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	MdCode   = lipgloss.NewStyle().Foreground(theme.Warn)
	mdFence  = lipgloss.NewStyle().Foreground(theme.OK)
	mdQuote  = lipgloss.NewStyle().Foreground(theme.Subtle).Italic(true)
	mdBullet = lipgloss.NewStyle().Foreground(theme.Primary)
)

// RenderMarkdown aplica um destaque leve, linha a linha: títulos, blocos de
// código, listas, citações e frontmatter. Sem dependência externa — é um
// leitor de SKILL.md, não um renderizador completo.
func RenderMarkdown(src string, width int) string {
	var b strings.Builder
	inFence := false
	inFront := false
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimRight(line, "\r")
		switch {
		case i == 0 && t == "---":
			inFront = true
			b.WriteString(mdQuote.Render(t))
		case inFront:
			if t == "---" || t == "..." {
				inFront = false
			}
			b.WriteString(mdQuote.Render(t))
		case strings.HasPrefix(t, "```"):
			inFence = !inFence
			b.WriteString(mdFence.Render(t))
		case inFence:
			b.WriteString(MdCode.Render(t))
		case strings.HasPrefix(t, "# "):
			b.WriteString(mdH1.Render(t))
		case strings.HasPrefix(t, "## "), strings.HasPrefix(t, "### "), strings.HasPrefix(t, "#### "):
			b.WriteString(mdH2.Render(t))
		case strings.HasPrefix(t, "> "):
			b.WriteString(mdQuote.Render(t))
		case strings.HasPrefix(strings.TrimSpace(t), "- "), strings.HasPrefix(strings.TrimSpace(t), "* "):
			indent := t[:len(t)-len(strings.TrimLeft(t, " \t"))]
			rest := strings.TrimSpace(t)[2:]
			b.WriteString(indent + mdBullet.Render("• ") + renderInline(rest))
		default:
			b.WriteString(renderInline(t))
		}
		b.WriteString("\n")
	}
	if width > 0 {
		return lipgloss.NewStyle().Width(width).Render(b.String())
	}
	return b.String()
}

// renderInline destaca `código inline` e **negrito** dentro de uma linha.
func renderInline(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i+1:], '`')
		if j < 0 {
			break
		}
		b.WriteString(renderBold(s[:i]))
		b.WriteString(MdCode.Render(s[i : i+j+2]))
		s = s[i+j+2:]
	}
	b.WriteString(renderBold(s))
	return b.String()
}

func renderBold(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "**")
		if i < 0 {
			break
		}
		j := strings.Index(s[i+2:], "**")
		if j < 0 {
			break
		}
		b.WriteString(s[:i])
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(s[i+2 : i+2+j]))
		s = s[i+j+4:]
	}
	b.WriteString(s)
	return b.String()
}
