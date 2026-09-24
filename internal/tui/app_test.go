package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
