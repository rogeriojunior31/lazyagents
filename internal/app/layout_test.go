package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// Exercise real modules without executing scans or commands against user data.
func TestResponsiveLayout(t *testing.T) {
	for _, size := range [][2]int{{40, 16}, {64, 24}, {80, 24}, {100, 40}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			d, err := LoadWith(core.PathsIn(t.TempDir()), "test")
			if err != nil {
				t.Fatal(err)
			}
			mods := d.Modules()
			var model tea.Model = tui.New(mods, nil, "test", tui.Options{})
			model, _ = model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			check := func(label string) {
				t.Helper()
				view := model.View().Content
				if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
					t.Fatalf("%s: got %dx%d", label, lipgloss.Width(view), lipgloss.Height(view))
				}
			}
			press := func(code rune) {
				model, _ = model.Update(tea.KeyPressMsg{Code: code})
			}
			check("splash")
			press(tea.KeyEnter)
			model, _ = model.Update(events.AgentsDetected{Agents: []agent.Agent{
				{ID: "codex", Name: "Codex", Installed: true, Version: "1.0"},
				{ID: "claude-code", Name: "Claude Code", Installed: true, Version: "1.0"},
				{ID: "gemini-cli", Name: "Gemini CLI"},
			}})
			for _, mod := range mods {
				check(mod.Title())
				if lipgloss.Width(mod.View()) > size[0]-4 {
					t.Fatalf("%s overflows available width", mod.Title())
				}
				press(tea.KeyRight)
				check("detail")
				press(tea.KeyLeft)
				press('?')
				check("help")
				// from 100 columns up, no help description is cut
				if size[0] >= 100 && strings.Contains(ansi.Strip(model.View().Content), "…") {
					t.Fatalf("%s: help truncated at %d columns", mod.Title(), size[0])
				}
				press(tea.KeyEscape)
				press(':')
				check("palette")
				press(tea.KeyEscape)
				if !strings.Contains(ansi.Strip(model.View().Content), "lazyagents") {
					t.Fatal("missing application header")
				}
				press(tea.KeyTab)
			}
		})
	}
}
