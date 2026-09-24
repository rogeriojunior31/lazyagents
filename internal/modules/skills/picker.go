package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// picker é a lista multi-select das skills descobertas numa origem de
// instalação (repositório GitHub, pasta ou zip).
type picker struct {
	items    []Found
	sel      map[int]bool
	cursor   int
	notesOff int
	origin   Origin // proveniência comum das skills descobertas
	cleanup  string // dir temporário para remover ao final
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

func (p picker) update(msg tea.KeyPressMsg, width, height int) picker {
	switch msg.String() {
	case "pgup", "pgdown":
		vp := p.notesViewport(width, height)
		vp, _ = vp.Update(msg)
		p.notesOff = vp.YOffset()
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	case "space":
		if p.cursor < len(p.items) {
			p.sel[p.cursor] = !p.sel[p.cursor]
		}
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
	start, end := p.window(width, maxH)
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
		line := fmt.Sprintf("%s %s %s  %s", mark, kind, name, kit.StHint.Render(f.Description))
		if i == p.cursor {
			line = lipgloss.NewStyle().Foreground(theme.Primary).Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(ansi.Truncate(line, max(1, width-4), "…") + "\n")
	}
	if len(p.origin.Notes) > 0 {
		vp := p.notesViewport(width, maxH)
		b.WriteString(fmt.Sprintf("\nAvisos · pgup/pgdn · %.0f%%\n", vp.ScrollPercent()*100) + vp.View() + "\n")
	}
	b.WriteString("space marca · a todas/nenhuma\nenter instala · esc volta")
	title := fmt.Sprintf("Instalar · %d/%d · %d marcados", min(p.cursor+1, len(p.items)), len(p.items), len(p.chosen()))
	return components.Panel{Title: title, Focused: true, Width: width}.Render(b.String())
}

func (p picker) notesViewport(width, height int) viewport.Model {
	content := ansi.Wrap(strings.Join(p.origin.Notes, "\n"), max(1, width-4), "")
	h := min(lipgloss.Height(content), max(1, height/3))
	vp := viewport.New(viewport.WithWidth(max(1, width-4)), viewport.WithHeight(h))
	vp.SetContent(content)
	vp.SetYOffset(p.notesOff)
	return vp
}

func (p picker) window(width, height int) (int, int) {
	rows := height - 4 // título, fundo e duas linhas de atalhos
	if len(p.origin.Notes) > 0 {
		rows -= 2 + p.notesViewport(width, height).Height()
	}
	return kit.Window(p.cursor, len(p.items), rows)
}
