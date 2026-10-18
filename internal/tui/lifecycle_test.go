package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"testing"
)

func TestCapturedInputBlocksTabHeaderNavigation(t *testing.T) {
	a := &capturingMod{fakeMod{id: "a"}}
	b := &fakeMod{id: "b"}
	m := New([]module.Module{a, b}, nil, "test", Options{NoSplash: true})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = updated.(Model)
	x := lipgloss.Width(m.navigation()[0].text) + 1
	updated, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: tabRowY(m.height)})
	m = updated.(Model)
	if m.active != 0 || cmd != nil {
		t.Fatal("header click escaped captured input")
	}
}

func TestPreferencesDiscardAndReloadPreserveDraftUntilConfirmed(t *testing.T) {
	m := New(nil, nil, "test", Options{NoSplash: true, LoadPreferences: func() (Preferences, error) { return preferenceFixture(), nil }})
	cmd := m.openPreferences()
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	m.width, m.height = 40, 16
	m.updatePreferences(tea.KeyPressMsg{Code: tea.KeyRight})
	p := m.preferences
	value := p.data.Theme
	m.updatePreferences(tea.KeyPressMsg{Code: tea.KeyEscape})
	if p.discard == nil {
		t.Fatal("dirty preferences discarded without confirmation")
	}
	m.updatePreferences(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.preferences != p || p.data.Theme != value || p.discard != nil {
		t.Fatal("cancel lost preference draft")
	}
	m.updatePreferences(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if p.discard == nil || !p.reload {
		t.Fatal("reload silently discarded changes")
	}
	reload := m.updatePreferences(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if reload == nil || m.preferences == p {
		t.Fatal("confirmed reload did not start")
	}
	updated, _ = m.Update(reload())
	m = updated.(Model)
	if preferencesDirty(m.preferences) || m.preferences.data.Theme != preferenceFixture().Theme {
		t.Fatal("reload retained old draft")
	}
}
