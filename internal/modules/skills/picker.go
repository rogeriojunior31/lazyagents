package skills

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// picker é a lista multi-select das skills descobertas numa origem de
// instalação (repositório GitHub, pasta ou zip).
type picker struct {
	items   []Found
	sel     map[int]bool
	cursor  int
	origin  Origin // proveniência comum das skills descobertas
	cleanup string // dir temporário para remover ao final
}

func newPicker(items []Found, origin Origin, cleanup string) picker {
	sel := make(map[int]bool, len(items))
	for i := range items {
		// Skills vêm marcadas (instalar o repo inteiro é 1 enter); hooks não:
		// um hook passa a rodar comando de terceiro a cada evento, então
		// entra só se o usuário marcar.
		sel[i] = items[i].Hook == nil
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

// hookCount conta as entradas que são hooks de plugin.
func (p picker) hookCount() int {
	n := 0
	for _, f := range p.items {
		if f.Hook != nil {
			n++
		}
	}
	return n
}

func (p picker) chosen() []Found {
	var out []Found
	for i, f := range p.items {
		if p.sel[i] {
			out = append(out, f)
		}
	}
	return out
}

func (p picker) view(width, maxH int) string {
	var b strings.Builder
	start, end := kit.Window(p.cursor, len(p.items), maxH-4-len(p.origin.Notes)-min(1, len(p.origin.Notes)))
	for i := start; i < end; i++ {
		f := p.items[i]
		mark := "[ ]"
		if p.sel[i] {
			mark = kit.StOn.Render("[x]")
		}
		name := f.Name
		if f.Plugin != "" && f.Hook == nil {
			name = kit.StHint.Render(f.Plugin+" › ") + name
		}
		if !f.Valid {
			name += " ⚠"
		}
		kind := "skill"
		if f.Hook != nil {
			// Hook roda comando de terceiro a cada evento: o tipo fica
			// explícito na lista, não escondido na descrição.
			kind = kit.StWarn.Render("hook ")
			name += kit.StHint.Render(fmt.Sprintf("  (%d) %s", len(f.Hook.Hooks), strings.Join(f.Hook.Events(), ", ")))
		} else {
			kind = kit.StHint.Render(kind)
		}
		line := fmt.Sprintf("%s %s %s  %s", mark, kind, name, kit.StHint.Render(kit.Truncate(f.Description, 50)))
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
	// avisos da descoberta (plugins do marketplace que vivem em outro repo)
	for _, n := range p.origin.Notes {
		b.WriteString("\n" + kit.StWarn.Render("! ") + kit.StHint.Render(kit.Truncate(n, width-8)))
	}
	if len(p.origin.Notes) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("\n" +
		components.Keycap("space") + kit.StHint.Render(" marca  ") +
		components.Keycap("a") + kit.StHint.Render(" todas/nenhuma  ") +
		components.Keycap("enter") + kit.StHint.Render(" instala  ") +
		components.Keycap("esc") + kit.StHint.Render(" cancela"))
	title := fmt.Sprintf("Skills encontradas (%d)", len(p.items))
	if nh := p.hookCount(); nh > 0 {
		title = fmt.Sprintf("Encontrados (%d skills, %d hooks)", len(p.items)-nh, nh)
	}
	return components.Panel{Title: title, Focused: true, Width: width}.Render(strings.TrimRight(b.String(), "\n"))
}
