package kit

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestHintsKeepHelpAndReturn(t *testing.T) {
	for _, width := range []int{0, 9, 20, 36, 76} {
		view := Hints(width, [2]string{"i", "install a skills repository"}, [2]string{"?", "help"}, [2]string{"esc", "back"}, [2]string{"?", "help"})
		plain := ansi.Strip(view)
		if lipgloss.Width(view) > width {
			t.Fatalf("hints exceed %d: %q", width, plain)
		}
		if width >= 9 && (!strings.Contains(plain, "?") || !strings.Contains(plain, "esc")) {
			t.Fatalf("help/back missing: %q", plain)
		}
		if strings.Count(plain, "?") > 1 {
			t.Fatal("duplicate help")
		}
	}
}
