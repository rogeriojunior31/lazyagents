package skills

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// picker is the multi-select list of skills found in an install source
// (GitHub repo, folder or zip).
type picker struct {
	items        []Found
	sel          map[int]bool
	cursor       int
	notesOff     int
	origin       Origin // shared provenance of the found skills
	cleanup      string // temp dir removed at the end
	agents       []agent.Agent
	ids          []string
	destinations bool
	targetCursor int
}

func newPicker(items []Found, origin Origin, cleanup string) picker {
	sel := make(map[int]bool, len(items))
	for i := range items {
		// Skills start checked; hooks require an individual opt-in:
		// a hook runs a third-party command on every event, so it is opt-in.
		sel[i] = items[i].Hook == nil
	}
	return picker{items: items, sel: sel, origin: origin, cleanup: cleanup}
}

func (p picker) update(msg tea.KeyPressMsg, width, height int) picker {
	if p.destinations {
		return p.updateDestinations(msg)
	}
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
			if p.items[i].Hook == nil && !p.sel[i] {
				all = false
				break
			}
		}
		for i := range p.items {
			if p.items[i].Hook == nil {
				p.sel[i] = !all
			}
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
	if p.destinations {
		return p.destinationsView(width, maxH)
	}
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
			// Hooks run third-party commands on every event: show the type, not just the description.
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
		b.WriteString(fmt.Sprintf("\nWarnings · pgup/pgdn · %.0f%%\n", vp.ScrollPercent()*100) + vp.View() + "\n")
	}
	if len(p.chosen()) == 0 {
		b.WriteString("Nothing selected · space marks\nenter targets · esc cancel")
	} else {
		b.WriteString("space mark · a skills all/none\nenter targets · esc cancel")
	}
	title := fmt.Sprintf("Install · %d/%d · %d marked", min(p.cursor+1, len(p.items)), len(p.items), len(p.chosen()))
	return components.Panel{Title: title, Focused: true, Width: width}.Render(b.String())
}

func (p picker) chosenSkills() int {
	n := 0
	for _, f := range p.chosen() {
		if f.Hook == nil {
			n++
		}
	}
	return n
}

func (p picker) destinationAgents() []agent.Agent {
	if p.chosenSkills() == 0 {
		return nil
	}
	return skillCapableAgents(p.agents)
}

func (p picker) updateDestinations(msg tea.KeyPressMsg) picker {
	targets := p.destinationAgents()
	switch msg.String() {
	case "up", "k":
		p.targetCursor = max(0, p.targetCursor-1)
	case "down", "j":
		p.targetCursor = min(len(targets), p.targetCursor+1)
	case "0", "x":
		p.ids = []string{}
	case "a":
		if len(p.ids) == len(targets) {
			p.ids = []string{}
			break
		}
		p.ids = []string{}
		for _, ag := range targets {
			p.ids = append(p.ids, ag.ID)
		}
	case "space":
		if p.targetCursor == 0 {
			p.ids = []string{}
			break
		}
		id := targets[p.targetCursor-1].ID
		if slices.Contains(p.ids, id) {
			p.ids = slices.DeleteFunc(p.ids, func(v string) bool { return v == id })
		} else {
			p.ids = append(p.ids, id)
		}
	}
	return p
}

func (p picker) destinationWindow(width, height int) (int, int) {
	return kit.Window(p.targetCursor, len(p.destinationAgents())+1, max(1, height-5-lipgloss.Height(p.destinationHints(width))))
}

func (p picker) destinationHints(width int) string {
	return kit.Wrap("space destination · a all/none\n0 library only · enter review\nesc skills", max(1, width-4), "")
}

func (p picker) destinationsView(width, height int) string {
	inner := max(1, width-4)
	skills := p.chosenSkills()
	lines := []string{
		ansi.Truncate(fmt.Sprintf("%d skills / %d hooks selected", skills, len(p.chosen())-skills), inner, "…"),
		ansi.Truncate(fmt.Sprintf("%d activation destinations", len(p.ids)), inner, "…"),
	}
	if skills == 0 {
		lines[1] = ansi.Truncate("Hooks: import only; enable in Hooks", inner, "…")
	}
	targets := p.destinationAgents()
	rows := []string{"Library only"}
	for _, ag := range targets {
		rows = append(rows, ag.Name)
	}
	start, end := p.destinationWindow(width, height)
	for i := start; i < end; i++ {
		checked := i == 0 && len(p.ids) == 0
		if i > 0 {
			checked = slices.Contains(p.ids, targets[i-1].ID)
		}
		mark := "[ ] "
		if checked {
			mark = "[x] "
		}
		prefix := "  "
		if i == p.targetCursor {
			prefix = "› "
		}
		line := ansi.Truncate(prefix+mark+rows[i], inner, "…")
		if i == p.targetCursor {
			line = kit.StTitle.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, p.destinationHints(width))
	return components.Panel{Title: "Install destinations", Focused: true, Width: width}.Render(strings.Join(lines, "\n"))
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
	rows := height - 4 // title, bottom and two hint lines
	if len(p.origin.Notes) > 0 {
		rows -= 2 + p.notesViewport(width, height).Height()
	}
	return kit.Window(p.cursor, len(p.items), rows)
}
