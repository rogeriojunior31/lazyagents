package agents

import (
	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"os"
	"strings"
	"testing"
)

func TestRefreshKeepsAgentIdentityAndDoesNotOverlap(t *testing.T) {
	m := newTab(t.TempDir(), nil)
	m.Update(events.AgentsDetected{Agents: []agent.Agent{{ID: "a"}, {ID: "b", Installed: true}}})
	calls := 0
	m.detect = func() agent.Detection {
		calls++
		return agent.Detection{Revision: 2, Agents: []agent.Agent{{ID: "a", Installed: true}, {ID: "b", Installed: true, DetectionError: "probe failed"}}}
	}
	cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	if cmd == nil || !m.refreshing || m.Update(refreshAgentsMsg{}) != nil {
		t.Fatal("refresh did not start once")
	}
	publish := m.Update(cmd())
	if publish == nil || m.refreshing || calls != 1 {
		t.Fatal("refresh completion did not publish a snapshot")
	}
	message := publish().(events.AgentsDetected)
	m.Update(message)
	if message.Revision != 2 || m.cursor != 1 || m.agents[m.cursor].ID != "b" || m.Count() != 2 {
		t.Fatalf("selection or counts changed incorrectly: %+v", m.agents)
	}
	if !strings.Contains(m.detailContent(m.agents[m.cursor], 80), "probe failed") || !strings.Contains(m.cells(m.agents[m.cursor])[0], "! ") {
		t.Fatal("failed probe is hidden")
	}
	m.doctor.open = true
	if cmd := m.Update(tea.KeyPressMsg{Code: 'r'}); cmd != nil && m.refreshing {
		t.Fatal("doctor retry refreshed detection")
	}
}

func TestFailedSessionRefreshPreservesAgentCounts(t *testing.T) {
	m := newTab(t.TempDir(), nil)
	m.Update(events.SessionsLoaded{Sessions: []agent.Session{{AgentID: "a"}}})
	m.Update(events.SessionsLoaded{Err: os.ErrPermission, LocalOnly: true})
	if m.counts["a"] != 1 {
		t.Fatal("failed session refresh erased the last completed count")
	}
	m.Update(events.SessionsLoaded{})
	if m.counts["a"] != 0 {
		t.Fatal("successful empty inventory did not clear the count")
	}
}
