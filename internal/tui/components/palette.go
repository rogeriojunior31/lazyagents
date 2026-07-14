package components

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/tui/theme"
)

// Command é uma ação nomeada exposta na paleta de comandos (estilo k9s: `:`).
type Command struct {
	Name string
	Desc string
}

// Palette é a paleta de comandos: input + lista filtrada por substring
// case-insensitive sobre o nome do comando. Quem chama decide o que cada
// Command.Name executa — este componente só sabe filtrar e navegar.
type Palette struct {
	input    textinput.Model
	commands []Command
	cursor   int
}

// NewPalette cria a paleta com o catálogo fixo de comandos disponíveis.
func NewPalette(cmds []Command) Palette {
	in := textinput.New()
	in.Placeholder = "comando…"
	in.Prompt = ": "
	in.CharLimit = 64
	in.SetWidth(40)
	return Palette{input: in, commands: cmds}
}

// Open reseta o texto/cursor e devolve o cmd de foco do textinput.
func (p Palette) Open() (Palette, tea.Cmd) {
	p.input.SetValue("")
	p.cursor = 0
	cmd := p.input.Focus()
	return p, cmd
}

// filtered devolve os comandos cujo nome contém o texto digitado; vazio
// devolve todos.
func (p Palette) filtered() []Command {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if q == "" {
		return p.commands
	}
	out := make([]Command, 0, len(p.commands))
	for _, c := range p.commands {
		if strings.Contains(strings.ToLower(c.Name), q) {
			out = append(out, c)
		}
	}
	return out
}

// Update processa uma tecla. done indica que a paleta deve fechar; choice é o
// nome do comando escolhido (vazio se fechou sem escolher, esc ou enter sem
// resultado filtrado).
func (p Palette) Update(msg tea.KeyPressMsg) (Palette, tea.Cmd, bool, string) {
	switch msg.String() {
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
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.cursor = 0
	}
	return p, cmd, false, ""
}

// View renderiza a paleta num Panel emoldurado, pronta para overlay central.
func (p Palette) View(width int) string {
	subtle := lipgloss.NewStyle().Foreground(theme.Subtle)
	cursor := lipgloss.NewStyle().Foreground(theme.Primary)

	f := p.filtered()
	var b strings.Builder
	b.WriteString(p.input.View() + "\n\n")
	if len(f) == 0 {
		b.WriteString(subtle.Render("nenhum comando"))
	}
	for i, c := range f {
		line := c.Name
		if c.Desc != "" {
			line += "  " + subtle.Render(c.Desc)
		}
		if i == p.cursor {
			line = cursor.Render("› ") + line
		} else {
			line = "  " + line
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	return Panel{Title: "Comandos", Focused: true, Width: width}.Render(b.String())
}
