package skills

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"strings"
	"testing"
)

func TestPickersFitAndClickVisibleItems(t *testing.T) {
	found := make([]Found, 30)
	repos := make([]RegistryResult, 30)
	for i := range found {
		found[i] = Found{Name: fmt.Sprintf("skill-%02d", i), Description: strings.Repeat("descrição ", 15), Valid: true}
		repos[i] = RegistryResult{Repo: fmt.Sprintf("repo-%02d", i), Description: found[i].Description}
	}
	found[29].Hook = &hooks.Found{}
	for _, size := range [][2]int{{36, 11}, {60, 19}, {96, 27}} {
		w, h := size[0], size[1]
		p := newPicker(found, Origin{Notes: []string{strings.Repeat("aviso longo ", 30) + "FIM DOS AVISOS"}}, "")
		if p.sel[29] {
			t.Fatal("hook veio marcado")
		}
		r := newRegistryPicker(repos)
		for i := range found {
			p.cursor, r.cursor = i, i
			for _, view := range []string{p.view(w, h), r.view(w, h)} {
				plain := ansi.Strip(view)
				if lipgloss.Width(view) > w || lipgloss.Height(view) > h || !strings.Contains(plain, "esc") || !strings.Contains(plain, "enter") {
					t.Fatalf("seletor fora da tela %v:\n%s", size, plain)
				}
			}
			if !strings.Contains(ansi.Strip(p.view(w, h)), found[i].Name) || !strings.Contains(ansi.Strip(r.view(w, h)), repos[i].Repo) {
				t.Fatal("seleção saiu da janela")
			}
		}
		for range 50 {
			p = p.update(tea.KeyPressMsg{Code: tea.KeyPgDown}, w, h)
		}
		if !strings.Contains(ansi.Strip(p.view(w, h)), "FIM DOS AVISOS") {
			t.Fatal("avisos não podem ser lidos até o fim")
		}
		start, _ := p.window(w, h)
		old := p.sel[start]
		m := Tab{width: w, height: h, mode: skModePick, picker: p}
		m, _ = m.click(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
		if m.picker.cursor != start || m.picker.sel[start] == old {
			t.Fatal("clique não alternou a entrada visível")
		}
		before := m.picker.cursor
		m, _ = m.click(tea.MouseClickMsg{X: 3, Y: h - 2, Button: tea.MouseLeft})
		if m.picker.cursor != before {
			t.Fatal("rodapé selecionou um item")
		}
		m.mode, m.regPicker = skModeRegistryPick, r
		start, _ = kit.Window(r.cursor, len(repos), h-4)
		m, _ = m.click(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
		if m.regPicker.cursor != start {
			t.Fatal("clique no resultado ignorou a rolagem")
		}
	}
}
