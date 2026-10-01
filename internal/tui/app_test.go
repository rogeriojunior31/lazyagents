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

// fakeMod records what it receives.
type fakeMod struct {
	id   string
	got  []tea.Msg
	keys int
}

func (f *fakeMod) ID() string               { return f.id }
func (f *fakeMod) Title() string            { return "Tab " + f.id }
func (f *fakeMod) Count() int               { return -1 }
func (f *fakeMod) Init() tea.Cmd            { return nil }
func (f *fakeMod) View() string             { return "body " + f.id }
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
	if !strings.Contains(view, "Tab a") || !strings.Contains(view, "body b") {
		t.Fatalf("first frame without tabs or off the start tab:\n%s", view)
	}
	var activated string
	for _, msg := range collect(m.Init()) {
		if e, ok := msg.(events.TabActivated); ok {
			activated = e.ID
		}
	}
	if activated != "b" {
		t.Errorf("TabActivated = %q, want b", activated)
	}
}

func TestBackgroundModuleGetsBroadcastsOnly(t *testing.T) {
	vis, bg := &fakeMod{id: "vis"}, &fakeMod{id: "bg"}
	var model tea.Model = New([]module.Module{vis}, nil, "t", Options{NoSplash: true, Background: []module.Module{bg}})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model, _ = model.Update(events.SessionsLoaded{})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'x'})
	if len(bg.got) != 2 || bg.keys != 0 || vis.keys != 1 {
		t.Errorf("hidden got %v (keys %d); visible keys %d", bg.got, bg.keys, vis.keys)
	}
	if strings.Contains(ansi.Strip(model.View().Content), "Tab bg") {
		t.Error("hidden tab shown in the bar")
	}
}

// collect runs a Cmd (and Batch) and returns the messages.
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
		keys[i] = [2]string{"x", "sample action"}
	}
	keys[len(keys)-1][1] = "last shortcut"
	return []module.HelpGroup{{Title: "Actions", Keys: keys}}
}

func TestHelpScrollAndReopen(t *testing.T) {
	mod := &longHelpMod{fakeMod{id: "test"}}
	var model tea.Model = New([]module.Module{mod}, nil, "test", Options{NoSplash: true})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	press := func(code rune) { model, _ = model.Update(tea.KeyPressMsg{Code: code}) }
	press('?')
	initial := ansi.Strip(model.View().Content)
	if strings.Contains(initial, "last shortcut") || !strings.Contains(initial, "esc") {
		t.Fatalf("unexpected initial help:\n%s", initial)
	}
	model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if model.(Model).helpScroll == 0 {
		t.Fatal("mouse wheel did not scroll the help")
	}
	for range 40 {
		press(tea.KeyDown)
	}
	if !strings.Contains(ansi.Strip(model.View().Content), "last shortcut") || mod.keys != 0 {
		t.Fatal("last shortcut unreachable or key sent to the tab")
	}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	if !strings.Contains(ansi.Strip(model.View().Content), "Navigation") {
		t.Fatal("resize did not adjust the scroll")
	}
	press(tea.KeyEscape)
	press('?')
	if model.(Model).helpScroll != 0 {
		t.Fatal("reopening must go back to the top")
	}
}

func TestNavigationWindowAndClicks(t *testing.T) {
	mods := []module.Module{}
	for _, name := range []string{"Skills", "Sessions", "Providers", "Hooks", "Usage", "Agents", "Plugin 界"} {
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
					t.Fatalf("click x=%d activates %d, want %d", x, next.(Model).active, item.index)
				}
				x += lipgloss.Width(item.text)
			}
			if x > width || !strings.Contains(ansi.Strip(row.String()), mods[active].Title()) {
				t.Fatalf("active tab cut at %d: %s", width, ansi.Strip(row.String()))
			}
			next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if next.(Model).active != (active+1)%len(mods) {
				t.Fatal("Tab changed the order")
			}
			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			if next.(Model).active != (active+len(mods)-1)%len(mods) {
				t.Fatal("Shift+Tab changed the order")
			}
		}
		if width == 40 && !strings.Contains(ansi.Strip(m.View().Content), "tab tabs") {
			t.Fatal("navigation hint disappeared")
		}
	}
}

func TestNavigationLongPluginAndEmpty(t *testing.T) {
	m := New(nil, nil, "test", Options{NoSplash: true})
	m.width = 40
	if len(m.navigation()) != 0 {
		t.Fatal("bar should be empty")
	}
	m.mods = []module.Module{&fakeMod{id: strings.Repeat("plugin", 30)}}
	items := m.navigation()
	if len(items) != 1 || lipgloss.Width(items[0].text) > 40 || !strings.Contains(ansi.Strip(items[0].text), "…") {
		t.Fatal("long plugin exceeds the width")
	}
}

func TestPalettePasteRoutesOnlyToPalette(t *testing.T) {
	a, b := &fakeMod{id: "a"}, &fakeMod{id: "target"}
	var model tea.Model = New([]module.Module{a, b}, nil, "t", Options{NoSplash: true})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	model, _ = model.Update(tea.KeyPressMsg{Code: ':'})
	before := len(a.got)
	model, _ = model.Update(tea.PasteMsg{Content: "target"})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.(Model).active != 1 || len(a.got) != before {
		t.Fatal("paste leaked out of the palette")
	}
}

type capturingMod struct{ fakeMod }

func (f *capturingMod) Capturing() bool { return true }

// 1-9 picks a visible tab; past the last tab, or while the tab owns the
// keyboard (a filter, a form), the digit is the tab's.
func TestDigitsSwitchTabs(t *testing.T) {
	a, b, bg := &fakeMod{id: "a"}, &fakeMod{id: "b"}, &fakeMod{id: "bg"}
	var model tea.Model = New([]module.Module{a, b}, nil, "t", Options{NoSplash: true, Background: []module.Module{bg}})
	press := func(r rune) tea.Cmd {
		var cmd tea.Cmd
		model, cmd = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		return cmd
	}
	if cmd := press('2'); model.(Model).active != 1 || len(collect(cmd)) != 1 {
		t.Fatalf("2: active %d", model.(Model).active)
	}
	press('3') // only two visible tabs: the hidden one is not reachable
	press('1')
	if model.(Model).active != 0 || a.keys+b.keys+bg.keys != 0 {
		t.Fatalf("active %d; keys reached a tab: %d %d %d", model.(Model).active, a.keys, b.keys, bg.keys)
	}

	in := &capturingMod{fakeMod{id: "in"}}
	model = New([]module.Module{in, b}, nil, "t", Options{NoSplash: true})
	press('2')
	if model.(Model).active != 0 || in.keys != 1 {
		t.Errorf("a capturing tab lost its digit: active %d, keys %d", model.(Model).active, in.keys)
	}
}
