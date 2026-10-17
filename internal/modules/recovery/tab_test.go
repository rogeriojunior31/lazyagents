package recovery

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestTabPreviewCancelAndRestore(t *testing.T) {
	svc, _, path, id := fixture(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	cmd := m.Update(events.TabActivated{ID: m.ID()})
	m.Update(cmd())
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	if m.preview == nil || m.preview.ID != id || strings.Contains(m.View(), "provider-secret") {
		t.Fatal("missing redacted preview")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.preview != nil {
		t.Fatal("escape did not cancel preview")
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
		t.Fatal("preview wrote the live file")
	}
	cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil {
		t.Fatal("restore skipped explicit confirmation")
	}
	cmd = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	result := cmd().(restoredMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	batch := m.Update(result)().(tea.BatchMsg)
	changed := false
	for _, command := range batch {
		msg := command()
		if event, ok := msg.(events.AgentConfigChanged); ok && event.AgentID == "claude-code" {
			changed = true
		}
		m.Update(msg)
	}
	if !changed {
		t.Fatal("other tabs were not told to reload config state")
	}
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "original-private") {
		t.Fatal("restore did not recover the selected backup")
	}
}

func TestRecoveryTabSearchAndSmallTerminals(t *testing.T) {
	svc, _, _, _ := fixture(t)
	m := newTab(svc)
	m.Update(m.loadCmd()())
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.PasteMsg{Content: "claude-code provider.apply"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.matches) != 1 {
		t.Fatal("agent/operation search failed")
	}
	for _, width := range []int{30, 40, 100} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		view := m.View()
		if lipgloss.Width(view) > width || lipgloss.Height(view) > 11 {
			t.Fatalf("list does not fit at %d: %s", width, view)
		}
	}
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	for _, width := range []int{30, 40, 100} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		view := m.View()
		if lipgloss.Width(view) > width || lipgloss.Height(view) > 11 || !strings.Contains(view, "esc") {
			t.Fatalf("preview does not fit at %d: %s", width, view)
		}
	}
}

func TestRecoveryReloadsOnReentry(t *testing.T) {
	svc, c, _, _ := fixture(t)
	m := newTab(svc)
	m.Update(m.Update(events.TabActivated{ID: m.ID()})())
	before := len(m.entries)
	if err := c.AddHook(agent.Hook{Event: agent.HookStop, Command: "echo new backup"}, svc.dir); err != nil {
		t.Fatal(err)
	}
	cmd := m.Update(events.TabActivated{ID: m.ID()})
	if cmd == nil {
		t.Fatal("returning to Recovery retained the stale backup inventory")
	}
	m.Update(cmd())
	if len(m.entries) != before+1 {
		t.Fatal("new snapshot not visible on reentry")
	}
}

func TestRecoveryExplainsUnverifiedBackupsWithoutDumpingIDs(t *testing.T) {
	svc, _, _, _ := fixture(t)
	m := newTab(svc)
	entries := []Entry{}
	for i := range 20 {
		entries = append(entries, Entry{ID: fmt.Sprintf("settings.json.20260923T163439.%09d", 818513061-i*2000000), Agent: "unknown", Operation: "unknown", Reason: "backup origin unavailable; legacy or damaged metadata"})
	}
	m.Update(loadedMsg{entries: entries})
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 45})
	view := ansi.Strip(m.View())
	for _, want := range []string{"Preview: 0", "Manual only: 20", "No backups ready to preview", "verified agent and destination"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "unknown") || strings.Contains(view, "settings.json.2026") || len(m.matches) != 0 {
		t.Fatal("unverified archive dump still presented as guided recovery")
	}
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if len(m.matches) != 20 || !strings.Contains(m.View(), "Guided restore unavailable") {
		t.Fatal("manual copies are inaccessible")
	}
	before, err := os.ReadFile(svc.adapters[0].(agent.ProviderHost).ProviderFile())
	if err != nil {
		t.Fatal(err)
	}
	if cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || m.inspection == nil || m.preview != nil || m.confirm != nil {
		t.Fatal("manual-only copy started a guided restore")
	}
	if !strings.Contains(m.View(), entries[0].ID) {
		t.Fatal("exact backup identity absent from details")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	after, err := os.ReadFile(svc.adapters[0].(agent.ProviderHost).ProviderFile())
	if err != nil || string(after) != string(before) {
		t.Fatal("inspection changed agent settings")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.inspection != nil || m.showUnavailable || len(m.matches) != 0 {
		t.Fatal("filter cancellation exposed hidden copies")
	}
}

func TestRecoveryMixedInventoryKeepsReadyCopiesVisible(t *testing.T) {
	svc, _, _, _ := fixture(t)
	entries, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	entries = append([]Entry{{ID: "settings.json.20260923T163439.818513061", Agent: "unknown", Operation: "unknown", Reason: "origin unavailable"}}, entries...)
	m := newTab(svc)
	m.Update(events.AgentsDetected{Agents: []agent.Agent{{ID: "claude-code", Name: "Claude Code"}}})
	m.Update(loadedMsg{entries: entries})
	for _, size := range [][2]int{{30, 11}, {40, 24}, {80, 24}, {120, 36}, {220, 64}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("inventory overflows %v", size)
		}
	}
	if len(m.matches) != 1 || !strings.Contains(m.View(), "Claude Code") || !strings.Contains(m.View(), "Apply provider") {
		t.Fatal("ready backup lacks context")
	}
	if strings.Contains(m.View(), "provider.apply") || strings.Contains(m.View(), "provider-secret") {
		t.Fatal("raw operation or secret exposed")
	}
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m.Update(tea.MouseClickMsg{X: 3, Y: lipgloss.Height(m.inventoryHeader()) + 3, Button: tea.MouseLeft})
	if m.cursor != 1 || !m.matches[m.cursor].Restorable {
		t.Fatal("table click did not select the available backup")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 11})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("available copy did not start preview verification")
	}
	m.Update(cmd())
	if m.preview == nil {
		t.Fatal("available copy could not be previewed")
	}
}
