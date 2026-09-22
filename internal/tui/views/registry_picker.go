package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// registryPickerState é a lista (seleção única) dos repositórios encontrados
// na busca do registry (M9.2). Escolher um entra no fluxo Discover/Install
// já existente (M8.B2), passando o repo como origem.
type registryPickerState struct {
	items  []skill.RegistryResult
	cursor int
}

func newRegistryPicker(items []skill.RegistryResult) registryPickerState {
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

func (p registryPickerState) selected() (skill.RegistryResult, bool) {
	if p.cursor < 0 || p.cursor >= len(p.items) {
		return skill.RegistryResult{}, false
	}
	return p.items[p.cursor], true
}

func (p registryPickerState) view(width, maxH int) string {
	var b strings.Builder
	start, end := window(p.cursor, len(p.items), maxH-4)
	for i := start; i < end; i++ {
		r := p.items[i]
		line := fmt.Sprintf("%s  %s", r.Repo, stHint.Render(truncate(r.Description, 60)))
		if i == p.cursor {
			line = lipgloss.NewStyle().Foreground(theme.Primary).Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	if end < len(p.items) {
		b.WriteString(stHint.Render(fmt.Sprintf("… mais %d", len(p.items)-end)) + "\n")
	}
	b.WriteString("\n" +
		components.Keycap("enter") + stHint.Render(" instala  ") +
		components.Keycap("esc") + stHint.Render(" cancela"))
	title := fmt.Sprintf("Resultados no GitHub (%d)", len(p.items))
	return components.Panel{Title: title, Focused: true, Width: width}.Render(strings.TrimRight(b.String(), "\n"))
}
