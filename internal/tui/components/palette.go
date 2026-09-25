package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Command is a named action in the command palette (k9s style: `:`).
type Command struct {
	Name string
	Desc string
}

// Palette filters commands by case-insensitive substring of the name. The caller
// decides what each Command.Name runs; this only filters and navigates.
type Palette struct {
	input    textinput.Model
	commands []Command
	cursor   int
}

// NewPalette builds the palette over a fixed command list.
func NewPalette(cmds []Command) Palette {
	in := NewInput()
	in.Placeholder = "command…"
	in.Prompt = ": "
	in.CharLimit = 64
	in.SetWidth(40)
	return Palette{input: in, commands: cmds}
}

// Open resets the text and returns the input focus cmd.
func (p Palette) Open() (Palette, tea.Cmd) {
	p.input.SetValue("")
	p.cursor = 0
	cmd := p.input.Focus()
	return p, cmd
}

func (p Palette) filtered() []Command {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if q == "" {
		return p.commands
	}
	out := make([]Command, 0, len(p.commands))
	for _, c := range p.commands {
		if strings.Contains(strings.ToLower(c.Name+" "+c.Desc), q) {
			out = append(out, c)
		}
	}
	return out
}

// Update handles a key. done closes the palette; choice is the chosen command
// name, empty on esc or enter with no match.
func (p Palette) Update(msg tea.Msg) (Palette, tea.Cmd, bool, string) {
	kp, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch kp.String() {
		case "esc":
			return p, nil, true, ""
		case "up", "ctrl+p":
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil, false, ""
		case "down", "ctrl+n":
			if f := p.filtered(); p.cursor < len(f)-1 {
				p.cursor++
			}
			return p, nil, false, ""
		case "enter":
			f := p.filtered()
			if p.cursor >= 0 && p.cursor < len(f) {
				return p, nil, true, f[p.cursor].Name
			}
			return p, nil, true, ""
		}
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.cursor = 0
	}
	return p, cmd, false, ""
}

// View renders the palette in a Panel, ready for a centered overlay.
func (p Palette) View(width, height int) string {
	subtle := lipgloss.NewStyle().Foreground(theme.Subtle)
	cursor := lipgloss.NewStyle().Foreground(theme.Primary)

	f := p.filtered()
	var b strings.Builder
	p.input.SetWidth(max(1, width-8))
	p.input.SetCursor(p.input.Position())
	b.WriteString(p.input.View() + "\n\n")
	if len(f) == 0 {
		b.WriteString(subtle.Render("no matching command"))
	}
	per := min(8, max(1, height-6))
	start := max(0, min(p.cursor-per/2, len(f)-per))
	end := min(len(f), start+per)
	nameW := 0 // descriptions aligned in one column
	for i := start; i < end; i++ {
		nameW = max(nameW, lipgloss.Width(f[i].Name))
	}
	for i := start; i < end; i++ {
		c := f[i]
		line := c.Name
		if c.Desc != "" {
			line += strings.Repeat(" ", nameW-lipgloss.Width(c.Name)+2) + subtle.Render(c.Desc)
		}
		if i == p.cursor {
			line = cursor.Background(theme.Sel).Bold(true).Width(max(1, width-4)).Render(
				ansi.Truncate("› "+ansi.Strip(line), max(1, width-4), "…"))
		} else {
			line = "  " + line
		}
		if i > start {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	hint := "↑↓ navigate · enter · esc close"
	if width >= 64 {
		hint = Keycap("↑↓") + subtle.Render(" navigate  ") + Keycap("enter") + subtle.Render(" run  ") + Keycap("esc") + subtle.Render(" close")
	}
	b.WriteString("\n\n" + hint)
	title := "Commands"
	if len(f) > 0 {
		title += fmt.Sprintf(" · %d/%d", p.cursor+1, len(f))
	}
	return Panel{Title: title, Focused: true, Width: width}.Render(b.String())
}
