package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLongConfirmKeepsActionsAndScrolls(t *testing.T) {
	for _, size := range [][2]int{{36, 11}, {60, 19}, {76, 19}} {
		w, h := size[0], size[1]
		c := NewConfirm(strings.Repeat("config file with a long path\n", 30) + "END OF CHANGE")
		check := func() string {
			t.Helper()
			view := c.ViewIn(w, h)
			plain := ansi.Strip(view)
			if lipgloss.Width(view) > w || lipgloss.Height(view) > h || !strings.Contains(plain, "Yes") || !strings.Contains(plain, "No") || !strings.Contains(plain, "esc") {
				t.Fatalf("actions off screen %v:\n%s", size, plain)
			}
			return plain
		}
		if strings.Contains(check(), "END OF CHANGE") {
			t.Fatal("fixture does not need scrolling")
		}
		c, _ = c.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown}, w, h)
		if c.scroll == 0 {
			t.Fatal("mouse did not scroll")
		}
		for range 100 {
			c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyDown}, w, h)
		}
		if !strings.Contains(check(), "END OF CHANGE") {
			t.Fatal("end of the question unreachable")
		}
		if _, res := c.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, w, h); res != No {
			t.Fatal("scrolling changed the default decision")
		}
		c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyLeft}, w, h)
		if _, res := c.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, w, h); res != Yes {
			t.Fatal("explicit selection does not confirm")
		}
		if !strings.Contains(ansi.Strip(c.ViewIn(100, 100)), "config file") {
			t.Fatal("resize lost content")
		}
	}
}
