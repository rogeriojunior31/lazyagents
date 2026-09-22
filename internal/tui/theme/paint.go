package theme

import (
	"image/color"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

var sgr = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// Paint restores the surrounding surface after nested styles reset ANSI colors.
// Lip Gloss closes each span with a full reset (\e[m) or a per-channel one
// (\e[39m / \e[49m); either would expose the terminal default inside a card.
// Every such reset is followed by the surface colors again, so inner spans can
// keep their own colors while the gaps between them stay painted.
func Paint(content string, foreground, background color.Color) string {
	fg := ansi.NewStyle().ForegroundColor(foreground).String()
	bg := ansi.NewStyle().BackgroundColor(background).String()
	content = sgr.ReplaceAllStringFunc(content, func(seq string) string {
		resetFg, resetBg := resets(sgr.FindStringSubmatch(seq)[1])
		if resetFg {
			seq += fg
		}
		if resetBg {
			seq += bg
		}
		return seq
	})
	// Each line is composable on its own: JoinHorizontal inserts a gap after
	// splitting cards into rows, which must not inherit the preceding surface.
	content = strings.ReplaceAll(content, "\n", ansi.ResetStyle+"\n"+fg+bg)
	return fg + bg + content + ansi.ResetStyle
}

// resets reports whether an SGR parameter list resets the foreground and/or
// background. Extended colors (38/48;5;n and 38/48;2;r;g;b) are skipped so a
// palette index like 49 is not mistaken for a reset.
func resets(params string) (fg, bg bool) {
	if params == "" {
		return true, true
	}
	ps := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	for i := 0; i < len(ps); i++ {
		switch ps[i] {
		case "0":
			fg, bg = true, true
		case "39":
			fg = true
		case "49":
			bg = true
		case "30", "31", "32", "33", "34", "35", "36", "37", "90", "91", "92", "93", "94", "95", "96", "97":
			fg = false // a later color in the same sequence wins over the reset
		case "40", "41", "42", "43", "44", "45", "46", "47", "100", "101", "102", "103", "104", "105", "106", "107":
			bg = false
		case "38", "48", "58":
			switch ps[i] {
			case "38":
				fg = false
			case "48":
				bg = false
			}
			if i+1 < len(ps) && ps[i+1] == "5" {
				i += 2
			} else if i+1 < len(ps) && ps[i+1] == "2" {
				i += 4
			}
		}
	}
	return fg, bg
}
