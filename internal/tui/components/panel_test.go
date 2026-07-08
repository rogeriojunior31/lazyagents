package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"lazyskills/internal/tui/theme"
)

func TestPanelDimensions(t *testing.T) {
	cases := []struct {
		name         string
		p            Panel
		content      string
		wantW, wantH int
	}{
		{"auto height, sem título", Panel{Width: 20}, "hi", 20, 3},    // 1 linha + 2 bordas
		{"auto height, 3 linhas", Panel{Width: 30}, "a\nb\nc", 30, 5}, // 3 + 2
		{"altura fixa maior que conteúdo", Panel{Width: 24, Height: 8}, "só uma", 24, 8},
		{"altura fixa menor trunca", Panel{Width: 24, Height: 4}, "1\n2\n3\n4\n5", 24, 4},
		{"com título", Panel{Width: 40, Title: "Skills (14)"}, "linha", 40, 3},
		{"título longo trunca mas largura mantém", Panel{Width: 16, Title: "um título absurdamente longo"}, "x", 16, 3},
		{"conteúdo mais largo que o painel", Panel{Width: 12}, strings.Repeat("x", 80), 12, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.p.Render(tc.content)
			if w := lipgloss.Width(out); w != tc.wantW {
				t.Errorf("largura = %d, quer %d\n%s", w, tc.wantW, out)
			}
			if h := lipgloss.Height(out); h != tc.wantH {
				t.Errorf("altura = %d, quer %d\n%s", h, tc.wantH, out)
			}
		})
	}
}

func TestPanelTitleVisible(t *testing.T) {
	out := Panel{Width: 40, Title: "Detalhe"}.Render("corpo")
	if !strings.Contains(out, "Detalhe") {
		t.Errorf("título não aparece:\n%s", out)
	}
}

func TestPanelBorderOverride(t *testing.T) {
	// A cor custom da borda não pode mexer nas dimensões nem sumir com o título.
	plain := Panel{Width: 30, Title: "você"}.Render("oi")
	tinted := Panel{Width: 30, Title: "você", Border: theme.OK}.Render("oi")
	if lipgloss.Width(plain) != lipgloss.Width(tinted) ||
		lipgloss.Height(plain) != lipgloss.Height(tinted) {
		t.Errorf("Border custom mudou as dimensões:\n%s\nvs\n%s", plain, tinted)
	}
	if !strings.Contains(tinted, "você") {
		t.Errorf("título sumiu com Border custom:\n%s", tinted)
	}
}

func TestPanelContentWidth(t *testing.T) {
	if got := (Panel{Width: 20}).ContentWidth(); got != 16 {
		t.Errorf("ContentWidth = %d, quer 16", got)
	}
	if got := (Panel{Width: 3}).ContentWidth(); got != 1 {
		t.Errorf("ContentWidth mínimo = %d, quer 1", got)
	}
}
