package theme

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestLiveTokensAndDefault(t *testing.T) {
	defer Apply(Default)
	if err := Apply(""); err != nil || Current() != "noite" {
		t.Fatal("Noite must be the default")
	}
	style := lipgloss.NewStyle().Foreground(Primary).Background(Bg)
	seen := map[string]bool{}
	for _, p := range Options() {
		if len(p.Colors) != 15 {
			t.Fatalf("%s: incomplete palette", p.ID)
		}
		if err := Apply(p.ID); err != nil {
			t.Fatal(err)
		}
		for key, value := range p.Colors {
			r, g, b, a := token(key).RGBA()
			wr, wg, wb, wa := lipgloss.Color(value).RGBA()
			if r != wr || g != wg || b != wb || a != wa {
				t.Fatalf("%s: %s did not update", p.ID, key)
			}
		}
		rendered := style.Render("same style, new theme")
		if seen[rendered] {
			t.Fatalf("cached style did not follow %s", p.ID)
		}
		seen[rendered] = true
	}
	before := Current()
	if err := Apply("invalid"); err == nil || Current() != before {
		t.Fatal("invalid theme changed the palette")
	}
}

func TestResetsIgnoresExtendedColors(t *testing.T) {
	cases := []struct {
		params string
		fg, bg bool
	}{
		{"", true, true},
		{"0", true, true},
		{"39", true, false},
		{"49", false, true},
		{"38;5;49", false, false}, // índice 49 da paleta, não reset
		{"48;2;0;49;39", false, false},
		{"0;38;5;3", false, true}, // cor posterior vence o reset
		{"1", false, false},
	}
	for _, c := range cases {
		fg, bg := resets(c.params)
		if fg != c.fg || bg != c.bg {
			t.Errorf("resets(%q) = %v,%v; quer %v,%v", c.params, fg, bg, c.fg, c.bg)
		}
	}
}

func TestPaintRestoresSurfaceAfterSpans(t *testing.T) {
	span := lipgloss.NewStyle().Foreground(Primary).Render("x")
	out := Paint(span+" y", Text, Surface)
	bg := "\x1b[48;2;"
	// depois do fechamento do span, o fundo precisa ser reaplicado antes de " y"
	i := strings.LastIndex(out, "x")
	if !strings.Contains(out[i:strings.LastIndex(out, " y")], bg) {
		t.Fatalf("fundo não reaplicado após o span: %q", out)
	}
}
