package kit

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ToastTTL is how long a toast stays visible.
const ToastTTL = 4 * time.Second

// Truncate cuts s to max runes, with an ellipsis.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// Window returns the visible range centered on the cursor.
func Window(cursor, total, size int) (int, int) {
	if size < 1 {
		size = 1
	}
	if total <= size {
		return 0, total
	}
	start := cursor - size/2
	if start < 0 {
		start = 0
	}
	if start+size > total {
		start = total - size
	}
	return start, start + size
}

// Wrap fits s in width columns with indent on each line, for card text whose
// end matters (path, error, endpoint), where truncating would hide it. Paths
// and URLs break after a "/" before resorting to a hard cut.
func Wrap(s string, width int, indent string) string {
	w := max(8, width-lipgloss.Width(indent))
	lines := strings.Split(ansi.Wrap(s, w, "/"), "\n")
	for i, ln := range lines {
		lines[i] = indent + strings.TrimRight(ln, " ")
	}
	return strings.Join(lines, "\n")
}

// Field is a card's "label  value" line, the value wrapped at its own column
// (labelW wide) so a long path keeps its end in sight.
func Field(label, value string, labelW, inner int) string {
	pad := strings.Repeat(" ", labelW)
	return CardLabel.Render(fmt.Sprintf("%-*s", labelW, label)) + strings.TrimPrefix(Wrap(value, inner, pad), pad)
}

// SideDetailWidth is the width from which table and detail sit side by side;
// below it the detail is a strip under the table.
const SideDetailWidth = 110

// Split divides a tab's area between table and detail.
type Split struct {
	Side             bool // detail on the right; otherwise a strip below
	ListW, ListH     int
	DetailW, DetailH int
}

// MaxDetailWidth caps the side detail, which otherwise takes 2/5 of the
// width: wider, its text runs too long to read, and the table, which holds the
// many rows, is the one worth growing.
const MaxDetailWidth = 88

// MaxStripHeight caps the detail strip under the table, so on tall narrow
// terminals (a portrait monitor) the table keeps most of the rows.
const MaxStripHeight = 12

// SplitDetail puts the detail beside the table when wide enough; otherwise
// table on top and a detail strip of a third of the height (3 to
// MaxStripHeight lines).
func SplitDetail(width, height int) Split {
	width, height = max(1, width), max(1, height)
	if width >= SideDetailWidth {
		dw := min(width*2/5, MaxDetailWidth)
		return Split{Side: true, ListW: width - dw - 2, ListH: height, DetailW: dw, DetailH: height}
	}
	dh := min(max(3, height/3), MaxStripHeight, max(0, height-3))
	return Split{ListW: width, ListH: height - dh, DetailW: width, DetailH: dh}
}

// Frame fits the body in height lines with the footer always on the last one:
// short bodies are padded, long ones cut before the footer.
func Frame(body, footer string, height int) string {
	foot := strings.Split(footer, "\n")
	if footer == "" {
		foot = nil
	}
	foot = foot[max(0, len(foot)-max(0, height)):]
	room := max(0, height-len(foot))
	lines := strings.Split(body, "\n")
	if len(lines) > room {
		lines = lines[:room]
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	return strings.Join(append(lines, foot...), "\n")
}
