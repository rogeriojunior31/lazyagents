package kit

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
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
// end matters (path, error, endpoint), where truncating would hide it.
func Wrap(s string, width int, indent string) string {
	w := max(8, width-lipgloss.Width(indent))
	lines := strings.Split(lipgloss.NewStyle().Width(w).Render(s), "\n")
	for i, ln := range lines {
		lines[i] = indent + strings.TrimRight(ln, " ")
	}
	return strings.Join(lines, "\n")
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

// MaxListWidth caps the table beside the detail: past it a flex column (a
// description, an endpoint) would only grow blank, so the detail gets the rest.
const MaxListWidth = 150

// SplitDetail puts the table at 3/5 (up to MaxListWidth) beside the detail
// when wide enough; otherwise table on top and a 3–6 line detail strip.
func SplitDetail(width, height int) Split {
	width, height = max(1, width), max(1, height)
	if width >= SideDetailWidth {
		lw := min(width*3/5, MaxListWidth)
		return Split{Side: true, ListW: lw, ListH: height, DetailW: width - lw - 2, DetailH: height}
	}
	dh := min(max(3, height/3), 6, max(0, height-3))
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
