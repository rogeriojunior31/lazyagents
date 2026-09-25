package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

func TestPanelDimensions(t *testing.T) {
	cases := []struct {
		name         string
		p            Panel
		content      string
		wantW, wantH int
	}{
		{"auto height, no title", Panel{Width: 20}, "hi", 20, 3},     // 1 line + 2 borders
		{"auto height, 3 lines", Panel{Width: 30}, "a\nb\nc", 30, 5}, // 3 + 2
		{"fixed height taller than content", Panel{Width: 24, Height: 8}, "just one", 24, 8},
		{"shorter fixed height truncates", Panel{Width: 24, Height: 4}, "1\n2\n3\n4\n5", 24, 4},
		{"with title", Panel{Width: 40, Title: "Skills (14)"}, "line", 40, 3},
		{"long title truncates, width holds", Panel{Width: 16, Title: "an absurdly long title"}, "x", 16, 3},
		{"content wider than the panel", Panel{Width: 12}, strings.Repeat("x", 80), 12, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.p.Render(tc.content)
			if w := lipgloss.Width(out); w != tc.wantW {
				t.Errorf("width = %d, want %d\n%s", w, tc.wantW, out)
			}
			if h := lipgloss.Height(out); h != tc.wantH {
				t.Errorf("height = %d, want %d\n%s", h, tc.wantH, out)
			}
		})
	}
}

func TestPanelTitleVisible(t *testing.T) {
	out := Panel{Width: 40, Title: "Detail"}.Render("body")
	if !strings.Contains(out, "Detail") {
		t.Errorf("title missing:\n%s", out)
	}
}

func TestPanelBorderOverride(t *testing.T) {
	// A custom border color must not change dimensions or hide the title.
	plain := Panel{Width: 30, Title: "you"}.Render("hi")
	tinted := Panel{Width: 30, Title: "you", Border: theme.OK}.Render("hi")
	if lipgloss.Width(plain) != lipgloss.Width(tinted) ||
		lipgloss.Height(plain) != lipgloss.Height(tinted) {
		t.Errorf("custom Border changed the dimensions:\n%s\nvs\n%s", plain, tinted)
	}
	if !strings.Contains(tinted, "you") {
		t.Errorf("title gone with custom Border:\n%s", tinted)
	}
}

func TestPanelContentWidth(t *testing.T) {
	if got := (Panel{Width: 20}).ContentWidth(); got != 16 {
		t.Errorf("ContentWidth = %d, want 16", got)
	}
	if got := (Panel{Width: 3}).ContentWidth(); got != 1 {
		t.Errorf("minimum ContentWidth = %d, want 1", got)
	}
}
