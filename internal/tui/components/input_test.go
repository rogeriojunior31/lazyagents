package components

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestInputViewKeepsCursorAndValue(t *testing.T) {
	in := NewInput()
	in.SetValue(strings.Repeat("caminho/", 30) + "FIM")
	in.Focus()
	in.CursorEnd()
	for _, width := range []int{16, 32, 68} {
		view := InputView(in, width)
		if lipgloss.Width(view) > width || !strings.Contains(ansi.Strip(view), "FIM") {
			t.Fatalf("cursor cortado: %s", ansi.Strip(view))
		}
		if !strings.HasSuffix(in.Value(), "FIM") {
			t.Fatal("renderização alterou valor")
		}
	}
}
