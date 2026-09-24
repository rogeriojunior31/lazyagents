package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderInlineBoldAroundCode(t *testing.T) {
	out := renderInline("**Badge `CXO` na Skills:** texto e `a**b`")
	plain := ansi.Strip(out)
	if plain != "Badge `CXO` na Skills: texto e `a**b`" {
		t.Errorf("texto = %q", plain)
	}
	if strings.Contains(plain, "**Badge") {
		t.Error("negrito com código dentro não foi aplicado")
	}
}

func TestTruncateSmallWidth(t *testing.T) {
	for _, width := range []int{-10, 0, 1, 2} {
		got := Truncate("ação", width)
		if len([]rune(got)) > max(0, width) {
			t.Errorf("Truncate largura %d = %q", width, got)
		}
	}
}
