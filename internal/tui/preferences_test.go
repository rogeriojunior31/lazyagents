package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

func preferenceFixture() Preferences {
	return Preferences{Theme: "sp-night", Themes: []string{"sp-night", "sp-night-garoa"}, StartTab: "skills", Revision: "original",
		Tabs: []PreferenceTab{{ID: "skills", Title: "Skills"}, {ID: "agents", Title: "Agents"}, {ID: "unknown", Title: "Unknown", Hidden: true, Unavailable: true}}}
}

func TestPreferencesEditingSaveAndModalOwnership(t *testing.T) {
	mod := &fakeMod{id: "skills"}
	saves := 0
	var saved Preferences
	m := New([]module.Module{mod}, nil, "test", Options{NoSplash: true,
		LoadPreferences: func() (Preferences, error) { return preferenceFixture(), nil },
		SavePreferences: func(p Preferences) (Preferences, error) { saves++; saved = p; p.Revision = "saved"; return p, nil },
	})
	beforeTheme := theme.Current()
	model, load := m.runPaletteCommand("settings")
	m = model.(Model)
	if m.preferences == nil || load == nil {
		t.Fatal("settings did not open")
	}
	model, _ = m.Update(load())
	m = model.(Model)
	press := func(code rune) { model, _ = m.Update(tea.KeyPressMsg{Code: code}); m = model.(Model) }
	press(tea.KeyRight)
	press(tea.KeyDown)
	press(tea.KeyRight)
	press(tea.KeyDown)
	press(tea.KeySpace)
	press(']')
	if m.preferences.data.Theme != "sp-night-garoa" || m.preferences.data.StartTab != "agents" || m.preferences.data.Tabs[1].ID != "skills" || !m.preferences.data.Tabs[1].Hidden {
		t.Fatalf("draft edits failed: %+v", m.preferences.data)
	}
	for _, size := range [][2]int{{40, 16}, {120, 36}} {
		model, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = model.(Model)
		view := m.View().Content
		if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] || !strings.Contains(ansi.Strip(view), "ctrl+s save") {
			t.Fatalf("preferences overflow or lack save action at %v: %s", size, ansi.Strip(view))
		}
	}
	inputs := len(mod.got)
	model, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 1})
	m = model.(Model)
	model, _ = m.Update(tea.PasteMsg{Content: "do not forward"})
	m = model.(Model)
	if mod.keys != 0 || len(mod.got) != inputs || m.preferences == nil {
		t.Fatal("preferences leaked input or closed on a tab click")
	}
	model, save := m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = model.(Model)
	press(tea.KeyEscape)
	if m.preferences == nil || !m.preferences.saving || save == nil {
		t.Fatal("in-flight save was discarded")
	}
	model, _ = m.Update(save())
	m = model.(Model)
	if saves != 1 || saved.Theme != "sp-night-garoa" || m.preferences.data.Revision != "saved" || !strings.Contains(m.preferences.message, "Saved.") || theme.Current() != beforeTheme {
		t.Fatal("save was lost, repeated or applied to the current session")
	}
	press(tea.KeyEscape)
	if m.preferences != nil || mod.keys != 0 {
		t.Fatal("closing settings forwarded input")
	}
}

func TestPreferencesRejectLastHiddenTabAndIgnoreLateLoads(t *testing.T) {
	m := New(nil, nil, "test", Options{NoSplash: true, LoadPreferences: func() (Preferences, error) { return preferenceFixture(), nil }})
	load := m.openPreferences()
	first := m.preferences
	m.updatePreferences(tea.KeyPressMsg{Code: tea.KeyEscape})
	secondLoad := m.openPreferences()
	model, _ := m.Update(load())
	m = model.(Model)
	if m.preferences == first || !m.preferences.loading {
		t.Fatal("late load replaced the newer editor")
	}
	model, _ = m.Update(secondLoad())
	m = model.(Model)
	m.preferences.data.Tabs = m.preferences.data.Tabs[:1]
	m.preferences.cursor = 2
	m.updatePreferences(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.preferences.data.Tabs[0].Hidden || m.preferences.message == "" {
		t.Fatal("last available tab was hidden")
	}
	for _, width := range []int{40, 120} {
		m.width, m.height = width, 16
		m.preferences.message = strings.Repeat("A config error with a very long path. ", 12) + "\x1b[31m"
		if view := m.View().Content; lipgloss.Width(view) != width || lipgloss.Height(view) != 16 || !strings.Contains(ansi.Strip(view), "ctrl+s save") {
			t.Fatal("error feedback hid save/cancel or exceeded the terminal")
		}
	}
	m.preferences.data.Tabs[0].Title = "unsafe\x1b[31m\nrow"
	if strings.Contains(preferencesText(m.preferences.data.Tabs[0].Title), "\x1b") {
		t.Fatal("terminal controls survived preference sanitization")
	}
	m.loadPreferences = func() (Preferences, error) { return Preferences{}, errors.New("Configuration unavailable") }
	load = m.openPreferences()
	model, _ = m.Update(load())
	m = model.(Model)
	if !strings.Contains(m.renderPreferences(), "Configuration unavailable") {
		t.Fatal("read failure was hidden")
	}
}
