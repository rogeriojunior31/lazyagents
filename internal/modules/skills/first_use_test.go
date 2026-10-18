package skills

import (
	tea "charm.land/bubbletea/v2"
	"testing"
)

func TestFirstUsePaletteOpensImportAndSearchWithoutFetching(t *testing.T) {
	m := newTab(New(testPaths(t)))
	for _, choice := range []struct {
		name string
		mode skMode
	}{{"install", skModeInstall}, {"search", skModeRegistry}} {
		found := false
		for _, command := range m.Commands() {
			if command.Name == choice.name {
				m.Update(command.Msg)
				found = true
				break
			}
		}
		if !found || m.mode != choice.mode || m.sourceOp != nil || m.inFlight {
			t.Fatalf("%s failed to open an idle input", choice.name)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
}
