package kit

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

var (
	mdH1     = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	mdH2     = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	MdCode   = lipgloss.NewStyle().Foreground(theme.SynString)
	mdFence  = lipgloss.NewStyle().Foreground(theme.SynPunct)
	mdQuote  = lipgloss.NewStyle().Foreground(theme.SynComment).Italic(true)
	mdBullet = lipgloss.NewStyle().Foreground(theme.Primary)
)

// RenderMarkdown aplica um destaque leve, linha a linha: títulos, blocos de
// código, listas, citações e frontmatter. Sem dependência externa — é um
// leitor de SKILL.md, não um renderizador completo.
func RenderMarkdown(src string, width int) string { return renderMarkdown(src, width, false) }

// RenderChat é RenderMarkdown para mensagem de conversa: as cercas de código
// saem (a de abertura vira o rótulo da linguagem), o código ganha recuo e as
// linhas não são completadas com espaço — o balão acompanha o texto.
func RenderChat(src string, width int) string {
	out := renderMarkdown(src, width, true)
	lines := strings.Split(out, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func renderMarkdown(src string, width int, chat bool) string {
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
			if !chat {
				b.WriteString(mdFence.Render(t))
				break
			}
			if !inFence {
				continue // cerca de fechamento: some
			}
			lang := strings.TrimSpace(strings.TrimPrefix(t, "```"))
			if lang == "" {
				lang = "código"
			}
			b.WriteString(mdFence.Render("  ▍" + lang))
		case inFence:
			if chat {
				t = "  " + t
			}
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

// renderInline destaca `código inline` e **negrito** dentro de uma linha,
// numa varredura só: negrito pode envolver código ("**a `b` c**"), e dentro
// do código ** é literal. Marcador sem par sai como texto.
func renderInline(s string) string {
	bold := lipgloss.NewStyle().Bold(true)
	var b strings.Builder
	var seg strings.Builder
	inBold := false
	emit := func() {
		if seg.Len() == 0 {
			return
		}
		if inBold {
			b.WriteString(bold.Render(seg.String()))
		} else {
			b.WriteString(seg.String())
		}
		seg.Reset()
	}
	for i := 0; i < len(s); {
		switch {
		case s[i] == '`':
			j := strings.IndexByte(s[i+1:], '`')
			if j < 0 {
				seg.WriteString(s[i:])
				i = len(s)
				continue
			}
			emit()
			code := MdCode
			if inBold {
				code = code.Bold(true)
			}
			b.WriteString(code.Render(s[i : i+j+2]))
			i += j + 2
		case strings.HasPrefix(s[i:], "**") && (inBold || strings.Contains(s[i+2:], "**")):
			emit()
			inBold = !inBold
			i += 2
		default:
			seg.WriteByte(s[i])
			i++
		}
	}
	emit()
	return b.String()
}
