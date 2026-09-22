package kit

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

// Hints keeps complete shortcuts on one line, leaving secondary actions in help.
func Hints(width int, actions ...[2]string) string {
	var parts []string
	used := 0
	for _, action := range actions {
		part := components.Keycap(action[0]) + StHint.Render(" "+action[1])
		w := lipgloss.Width(part)
		if len(parts) > 0 {
			w += 2
		}
		if used+w > width {
			break
		}
		parts = append(parts, part)
		used += w
	}
	return strings.Join(parts, "  ")
}
