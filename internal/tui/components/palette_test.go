package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestPaletteSmallTerminalAndPaste(t *testing.T) {
	commands := make([]Command, 30)
	for i := range commands {
		commands[i] = Command{Name: fmt.Sprintf("comando-%02d", i), Desc: "descrição"}
	}
	for _, size := range [][2]int{{36, 11}, {60, 19}, {76, 19}} {
		p := NewPalette(commands)
		p, _ = p.Open()
		for i := range commands {
			view := p.View(size[0], size[1])
			plain := ansi.Strip(view)
			if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] || !strings.Contains(plain, "› "+commands[i].Name) || !strings.Contains(plain, "esc") {
				t.Fatalf("seleção cortada:\n%s", plain)
			}
			p, _, _, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		p, _, _, _ = p.Update(tea.PasteMsg{Content: "comando-04"})
		if p.cursor != 0 {
			t.Fatal("paste não reiniciou seleção")
		}
		_, _, done, choice := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if !done || choice != "comando-04" {
			t.Fatal("comando colado não foi selecionado")
		}
		p, _ = p.Open()
		p, _, _, _ = p.Update(tea.PasteMsg{Content: strings.Repeat("x", 55) + "FIM"})
		if !strings.Contains(ansi.Strip(p.View(size[0], size[1])), "FIM") {
			t.Fatal("cursor de texto longo cortado")
		}
	}
}
