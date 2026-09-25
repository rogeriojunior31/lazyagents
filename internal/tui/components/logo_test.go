package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestLogoArtSize(t *testing.T) {
	art := logoArt()
	lines := strings.Split(art, "\n")
	if len(lines) != 8 {
		t.Fatalf("got %d lines, want 8", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 30 {
			t.Fatalf("line %d: width %d, want 30", i, w)
		}
	}
}

func TestSplashShowsArtOnlyWhenItFits(t *testing.T) {
	for _, c := range []struct {
		w, h int
		art  bool
	}{{120, 40, true}, {80, 24, true}, {80, 22, true}, {80, 21, false}, {60, 40, false}} {
		view := NewSplash("test").Resize(c.w, c.h).View()
		if got := strings.Contains(view, "▀"); got != c.art {
			t.Errorf("%dx%d: art=%v, want %v", c.w, c.h, got, c.art)
		}
		if lipgloss.Width(view) != c.w || lipgloss.Height(view) != c.h {
			t.Errorf("%dx%d: view %dx%d", c.w, c.h, lipgloss.Width(view), lipgloss.Height(view))
		}
	}
}
