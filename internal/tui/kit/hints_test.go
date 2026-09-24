package kit

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestHintsKeepHelpAndReturn(t *testing.T) {
	for _, width := range []int{0, 9, 20, 36, 76} {
		view := Hints(width, [2]string{"i", "instalar um repositório de skills"}, [2]string{"?", "atalhos"}, [2]string{"esc", "volta"}, [2]string{"?", "atalhos"})
		plain := ansi.Strip(view)
		if lipgloss.Width(view) > width {
			t.Fatalf("hints excede %d: %q", width, plain)
		}
		if width >= 9 && (!strings.Contains(plain, "?") || !strings.Contains(plain, "esc")) {
			t.Fatalf("ajuda/retorno sumiram: %q", plain)
		}
		if strings.Count(plain, "?") > 1 {
			t.Fatal("ajuda duplicada")
		}
	}
}
