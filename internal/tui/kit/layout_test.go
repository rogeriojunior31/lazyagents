package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderInlineBoldAroundCode(t *testing.T) {
	out := renderInline("**Badge `CXO` in Skills:** text and `a**b`")
	plain := ansi.Strip(out)
	if plain != "Badge `CXO` in Skills: text and `a**b`" {
		t.Errorf("text = %q", plain)
	}
	if strings.Contains(plain, "**Badge") {
		t.Error("bold wrapping code not applied")
	}
}

func TestTruncateSmallWidth(t *testing.T) {
	for _, width := range []int{-10, 0, 1, 2} {
		got := Truncate("ação", width) // check-english:allow — multi-byte runes
		if len([]rune(got)) > max(0, width) {
			t.Errorf("Truncate width %d = %q", width, got)
		}
	}
}
