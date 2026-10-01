package kit

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderChatTable(t *testing.T) {
	src := "Before\n| Area | Pi | How it fits |\n|---|:-:|---|\n| Skills | `~/.pi/skills` | " +
		strings.Repeat("a long cell that must wrap ", 4) + "|\n| Short |\nAfter"
	out := RenderChat(src, 50)
	plain := ansi.Strip(out)
	for _, want := range []string{"Before", "After", "│ Area", "│ Skills", "~/.pi/skill", "╭", "╰"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "|---") || strings.Contains(plain, "| Area") {
		t.Errorf("table left raw:\n%s", plain)
	}
	if w := lipgloss.Width(out); w > 50 {
		t.Errorf("width = %d, should fit 50:\n%s", w, plain)
	}
}

// Pipes without a separator row are text, not a table.
func TestRenderChatPipesWithoutSeparator(t *testing.T) {
	plain := ansi.Strip(RenderChat("| a | b |\n| c | d |", 40))
	if !strings.Contains(plain, "| a | b |") {
		t.Errorf("pipes should stay as text:\n%s", plain)
	}
}
