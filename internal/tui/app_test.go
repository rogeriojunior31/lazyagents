package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// fakeMod registra o que recebeu.
type fakeMod struct {
	id   string
	got  []tea.Msg
	keys int
}

func (f *fakeMod) ID() string               { return f.id }
func (f *fakeMod) Title() string            { return "Aba " + f.id }
func (f *fakeMod) Count() int               { return -1 }
func (f *fakeMod) Init() tea.Cmd            { return nil }
func (f *fakeMod) View() string             { return "corpo " + f.id }
func (f *fakeMod) Capturing() bool          { return false }
func (f *fakeMod) ClearToast()              {}
func (f *fakeMod) Help() []module.HelpGroup { return nil }
func (f *fakeMod) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		f.keys++
	}
	f.got = append(f.got, msg)
	return nil
}

func TestNoSplashStartsOnStartTab(t *testing.T) {
	a, b := &fakeMod{id: "a"}, &fakeMod{id: "b"}
	m := New([]module.Module{a, b}, nil, "t", Options{NoSplash: true, Start: 1})
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "Aba a") || !strings.Contains(view, "corpo b") {
		t.Fatalf("primeiro frame sem abas ou fora da aba inicial:\n%s", view)
	}
	var activated string
	for _, msg := range collect(m.Init()) {
		if e, ok := msg.(events.TabActivated); ok {
			activated = e.ID
		}
	}
	if activated != "b" {
		t.Errorf("TabActivated = %q, quer b", activated)
	}
}

func TestBackgroundModuleGetsBroadcastsOnly(t *testing.T) {
	vis, bg := &fakeMod{id: "vis"}, &fakeMod{id: "bg"}
	var model tea.Model = New([]module.Module{vis}, nil, "t", Options{NoSplash: true, Background: []module.Module{bg}})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model, _ = model.Update(events.SessionsLoaded{})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'x'})
	if len(bg.got) != 2 || bg.keys != 0 || vis.keys != 1 {
		t.Errorf("oculta recebeu %v (teclas %d); visível teclas %d", bg.got, bg.keys, vis.keys)
	}
	if strings.Contains(ansi.Strip(model.View().Content), "Aba bg") {
		t.Error("aba oculta apareceu na barra")
	}
}

// collect roda um Cmd (e Batch) e devolve as mensagens produzidas.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

type longHelpMod struct{ fakeMod }

func (f *longHelpMod) Help() []module.HelpGroup {
	keys := make([][2]string, 30)
	for i := range keys {
		keys[i] = [2]string{"x", "ação de exemplo"}
	}
	keys[len(keys)-1][1] = "último atalho"
	return []module.HelpGroup{{Title: "Ações", Keys: keys}}
}

func TestHelpScrollAndReopen(t *testing.T) {
	mod := &longHelpMod{fakeMod{id: "teste"}}
	var model tea.Model = New([]module.Module{mod}, nil, "test", Options{NoSplash: true})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	press := func(code rune) { model, _ = model.Update(tea.KeyPressMsg{Code: code}) }
	press('?')
	initial := ansi.Strip(model.View().Content)
	if strings.Contains(initial, "último atalho") || !strings.Contains(initial, "esc") {
		t.Fatalf("ajuda inicial inesperada:\n%s", initial)
	}
	model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if model.(Model).helpScroll == 0 {
		t.Fatal("roda do mouse não rolou a ajuda")
	}
	for range 40 {
		press(tea.KeyDown)
	}
	if !strings.Contains(ansi.Strip(model.View().Content), "último atalho") || mod.keys != 0 {
		t.Fatal("último atalho inacessível ou tecla entregue à aba")
	}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	if !strings.Contains(ansi.Strip(model.View().Content), "Navegação") {
		t.Fatal("resize não ajustou a rolagem")
	}
	press(tea.KeyEscape)
	press('?')
	if model.(Model).helpScroll != 0 {
		t.Fatal("reabrir deve voltar ao topo")
	}
}

func TestNavigationWindowAndClicks(t *testing.T) {
	mods := []module.Module{}
	for _, name := range []string{"Skills", "Sessões", "Provedores", "Hooks", "Uso", "Agentes", "Plugin 界"} {
		mods = append(mods, &fakeMod{id: name})
	}
	for _, width := range []int{40, 64, 80, 120, 200} {
		m := New(mods, nil, "test", Options{NoSplash: true})
		m.width, m.height = width, 24
		for active := range mods {
			m.active = active
			items := m.navigation()
			var row strings.Builder
			x := 0
			for _, item := range items {
				row.WriteString(item.text)
				next, _ := m.Update(tea.MouseClickMsg{X: x, Y: tabRowY, Button: tea.MouseLeft})
				if next.(Model).active != item.index {
					t.Fatalf("clique x=%d ativa %d, queria %d", x, next.(Model).active, item.index)
				}
				x += lipgloss.Width(item.text)
			}
			if x > width || !strings.Contains(ansi.Strip(row.String()), mods[active].Title()) {
				t.Fatalf("aba ativa cortada em %d: %s", width, ansi.Strip(row.String()))
			}
			next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if next.(Model).active != (active+1)%len(mods) {
				t.Fatal("Tab alterou a ordem")
			}
			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			if next.(Model).active != (active+len(mods)-1)%len(mods) {
				t.Fatal("Shift+Tab alterou a ordem")
			}
		}
		if width == 40 && !strings.Contains(ansi.Strip(m.View().Content), "tab abas") {
			t.Fatal("atalho de navegação desapareceu")
		}
	}
}

func TestNavigationLongPluginAndEmpty(t *testing.T) {
	m := New(nil, nil, "test", Options{NoSplash: true})
	m.width = 40
	if len(m.navigation()) != 0 {
		t.Fatal("barra deveria estar vazia")
	}
	m.mods = []module.Module{&fakeMod{id: strings.Repeat("plugin", 30)}}
	items := m.navigation()
	if len(items) != 1 || lipgloss.Width(items[0].text) > 40 || !strings.Contains(ansi.Strip(items[0].text), "…") {
		t.Fatal("plugin longo excede largura")
	}
}

func TestPalettePasteRoutesOnlyToPalette(t *testing.T) {
	a, b := &fakeMod{id: "a"}, &fakeMod{id: "destino"}
	var model tea.Model = New([]module.Module{a, b}, nil, "t", Options{NoSplash: true})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	model, _ = model.Update(tea.KeyPressMsg{Code: ':'})
	before := len(a.got)
	model, _ = model.Update(tea.PasteMsg{Content: "destino"})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.(Model).active != 1 || len(a.got) != before {
		t.Fatal("paste não foi isolado na paleta")
	}
}
