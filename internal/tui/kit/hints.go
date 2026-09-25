package kit

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

// Hints keeps help/back (? and esc) visible before secondary actions, never cutting a key.
func Hints(width int, actions ...[2]string) string {
	render := func(action [2]string) string {
		return components.Keycap(action[0]) + StHint.Render(" "+action[1])
	}
	var essential, parts []string
	seen := map[string]bool{}
	for _, action := range actions {
		if (action[0] == "?" || action[0] == "esc") && !seen[action[0]] {
			essential = append(essential, render(action))
			seen[action[0]] = true
		}
	}
	suffix := strings.Join(essential, "  ")
	if lipgloss.Width(suffix) > width {
		essential = nil
		for _, action := range actions {
			if seen[action[0]] {
				essential = append(essential, components.Keycap(action[0]))
				delete(seen, action[0])
			}
		}
		suffix = strings.Join(essential, " ")
	}
	used := lipgloss.Width(suffix)
	for _, action := range actions {
		if action[0] == "?" || action[0] == "esc" {
			continue
		}
		part := render(action)
		w := lipgloss.Width(part)
		if used > 0 {
			w += 2
		}
		if used+w > width {
			continue
		}
		parts = append(parts, part)
		used += w
	}
	if suffix != "" && lipgloss.Width(suffix) <= width {
		parts = append(parts, suffix)
	}
	return strings.Join(parts, "  ")
}
