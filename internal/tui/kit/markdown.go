package kit

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

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

// RenderMarkdown highlights line by line: headings, code blocks, lists, quotes
// and frontmatter. A SKILL.md reader, not a full renderer.
func RenderMarkdown(src string, width int) string { return renderMarkdown(src, width, false) }

// RenderChat is RenderMarkdown for chat messages: fences are dropped (the
// opening one becomes the language label), code is indented and lines are not
// padded, so the bubble follows the text.
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
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		t := strings.TrimRight(lines[i], "\r")
		if chat && !inFence && !inFront && tableRow(t) {
			j := i + 1
			for j < len(lines) && tableRow(strings.TrimRight(lines[j], "\r")) {
				j++
			}
			if tbl, ok := renderTable(lines[i:j], width); ok {
				b.WriteString(tbl + "\n")
				i = j - 1
				continue
			}
		}
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
				continue // closing fence is dropped
			}
			lang := strings.TrimSpace(strings.TrimPrefix(t, "```"))
			if lang == "" {
				lang = "code"
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

// renderInline highlights `code` and **bold** in one pass: bold may wrap code
// ("**a `b` c**") and ** is literal inside code. Unpaired markers stay as text.
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

func tableRow(line string) bool {
	t := strings.TrimSpace(line)
	return len(t) > 1 && t[0] == '|' && t[len(t)-1] == '|'
}

// renderTable draws a GitHub table (header, |---| separator, rows) fitted to
// width, cells wrapping. Without the separator it is not a table: ok=false.
func renderTable(lines []string, width int) (string, bool) {
	if len(lines) < 2 || strings.Trim(strings.TrimSpace(lines[1]), "|-: ") != "" {
		return "", false
	}
	cells := func(line string) []string {
		t := strings.TrimSpace(strings.TrimRight(line, "\r"))
		parts := strings.Split(t[1:len(t)-1], "|")
		for i, p := range parts {
			parts[i] = renderInline(strings.TrimSpace(p))
		}
		return parts
	}
	header := cells(lines[0])
	tb := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(mdFence).
		Headers(header...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				s = s.Inherit(mdH2)
			}
			return s
		})
	for _, ln := range lines[2:] {
		row := cells(ln)
		// Ragged rows are padded or cut to the header's columns.
		row = append(row, make([]string, max(0, len(header)-len(row)))...)
		tb.Row(row[:len(header)]...)
	}
	if width > 0 && lipgloss.Width(tb.String()) > width {
		tb.Width(width)
	}
	return tb.String(), true
}
