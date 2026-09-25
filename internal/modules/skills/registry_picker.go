package skills

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// registryPickerState é a lista (seleção única) dos repositórios encontrados
// na busca do registry. Escolher um entra no fluxo Discover/Install
// já existente, passando o repo como origem.
type registryPickerState struct {
	items  []RegistryResult
	cursor int
}

func newRegistryPicker(items []RegistryResult) registryPickerState {
	return registryPickerState{items: items}
}

func (p *registryPickerState) update(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	}
}

func (p registryPickerState) selected() (RegistryResult, bool) {
	if p.cursor < 0 || p.cursor >= len(p.items) {
		return RegistryResult{}, false
	}
	return p.items[p.cursor], true
}

func (p registryPickerState) view(width, maxH int) string {
	var b strings.Builder
	start, end := kit.Window(p.cursor, len(p.items), maxH-4)
	for i := start; i < end; i++ {
		r := p.items[i]
		line := fmt.Sprintf("%s  %s", r.Repo, kit.StHint.Render(r.Description))
		if i == p.cursor {
			line = lipgloss.NewStyle().Foreground(theme.Primary).Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(ansi.Truncate(line, max(1, width-4), "…") + "\n")
	}
	b.WriteString("\n" + kit.Hints(width-4, [2]string{"enter", "install"}, [2]string{"esc", "back"}))
	title := fmt.Sprintf("GitHub · %d/%d", min(p.cursor+1, len(p.items)), len(p.items))
	return components.Panel{Title: title, Focused: true, Width: width}.Render(b.String())
}
