package skills

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// picker é a lista multi-select das skills descobertas numa origem de
// instalação (repositório GitHub, pasta ou zip).
type picker struct {
	items   []skill.Found
	sel     map[int]bool
	cursor  int
	origin  skill.Origin // proveniência comum das skills descobertas
	cleanup string       // dir temporário para remover ao final
}

func newPicker(items []skill.Found, origin skill.Origin, cleanup string) picker {
	sel := make(map[int]bool, len(items))
	for i := range items {
		sel[i] = true // tudo marcado por padrão: instalar o repo inteiro é 1 enter
	}
	return picker{items: items, sel: sel, origin: origin, cleanup: cleanup}
}

func (p picker) update(msg tea.KeyPressMsg) picker {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	case "space":
		p.sel[p.cursor] = !p.sel[p.cursor]
	case "a":
		all := true
		for i := range p.items {
			if !p.sel[i] {
				all = false
				break
			}
		}
		for i := range p.items {
			p.sel[i] = !all
		}
	}
	return p
}

func (p picker) chosen() []skill.Found {
	var out []skill.Found
	for i, f := range p.items {
		if p.sel[i] {
			out = append(out, f)
		}
	}
	return out
}

func (p picker) view(width, maxH int) string {
	var b strings.Builder
	start, end := kit.Window(p.cursor, len(p.items), maxH-4)
	for i := start; i < end; i++ {
		f := p.items[i]
		mark := "[ ]"
		if p.sel[i] {
			mark = kit.StOn.Render("[x]")
		}
		name := f.Name
		if !f.Valid {
			name += " ⚠"
		}
		line := fmt.Sprintf("%s %s  %s", mark, name, kit.StHint.Render(kit.Truncate(f.Description, 60)))
		if i == p.cursor {
			line = lipgloss.NewStyle().Foreground(theme.Primary).Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	if end < len(p.items) {
		b.WriteString(kit.StHint.Render(fmt.Sprintf("… mais %d", len(p.items)-end)) + "\n")
	}
	b.WriteString("\n" +
		components.Keycap("space") + kit.StHint.Render(" marca  ") +
		components.Keycap("a") + kit.StHint.Render(" todas/nenhuma  ") +
		components.Keycap("enter") + kit.StHint.Render(" instala  ") +
		components.Keycap("esc") + kit.StHint.Render(" cancela"))
	title := fmt.Sprintf("Skills encontradas (%d)", len(p.items))
	return components.Panel{Title: title, Focused: true, Width: width}.Render(strings.TrimRight(b.String(), "\n"))
}
