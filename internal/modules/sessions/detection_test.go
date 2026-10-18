package sessions

import (
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestDetectionReloadPreservesSessionIdentityAndStaysLocal(t *testing.T) {
	m := tableTab(t, 3, 100, 24)
	m.list.Select(1)
	selected := m.list.SelectedItem().(sessionItem).s
	cmd := m.Update(events.AgentsDetected{Revision: 2})
	result := cmd().(events.SessionsLoaded)
	if !result.LocalOnly || result.Revision != 2 {
		t.Fatal("detection reload lost its local scope or revision")
	}
	newSession := agent.Session{ID: "new", AgentID: "a", Title: "Inserted before selection"}
	m.Update(events.SessionsLoaded{Sessions: append([]agent.Session{newSession}, m.sessions...), LocalOnly: true, Revision: 2})
	if got := m.list.SelectedItem().(sessionItem).s; aliasKey(got) != aliasKey(selected) {
		t.Fatalf("session identity changed: %+v -> %+v", selected, got)
	}
}
