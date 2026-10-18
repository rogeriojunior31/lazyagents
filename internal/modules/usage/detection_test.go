package usage

import (
	"os"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestDetectionSessionReloadNeverFetchesLimits(t *testing.T) {
	calls := 0
	ad := fakeAdapter{id: "x", calls: &calls, auth: agent.AuthAPIKey, events: []agent.UsageEvent{{AgentID: "x", N: 1}}}
	m := newTab(New([]agent.Adapter{ad}, core.PathsIn(t.TempDir())), config{})
	m.loaded = true
	m.f.text = "keep filter"
	m.statuses = []Status{{AgentID: "x", AuthLabel: "previous limits"}}
	m.Update(events.AgentsDetected{Agents: []agent.Agent{{ID: "x", Name: "New name"}}, Revision: 2})
	cmd := m.Update(events.SessionsLoaded{Sessions: []agent.Session{{ID: "s", AgentID: "x"}}, Revision: 2, LocalOnly: true})
	if cmd == nil {
		t.Fatal("local usage aggregation was not scheduled")
	}
	m.Update(cmd())
	if calls != 0 || len(m.events) != 1 || len(m.statuses) != 1 || !m.api["x"] || m.f.text != "keep filter" || m.names["x"] != "New name" || m.loading {
		t.Fatal("refresh fetched limits, erased evidence/filter or missed local data")
	}
	m.pending, m.loading = 1, true
	oldRequest := m.eventsRequest
	m.eventsRequest++
	m.Update(eventsMsg{request: oldRequest})
	if len(m.events) != 1 || m.loading {
		t.Fatal("an obsolete aggregation erased evidence or left the loading state stuck")
	}
}

func TestFailedSessionRefreshPreservesUsageEvidence(t *testing.T) {
	m := newTab(New(nil, core.PathsIn(t.TempDir())), config{})
	m.loaded = true
	m.sessions = []agent.Session{{ID: "keep", AgentID: "x"}}
	m.events = []agent.UsageEvent{{AgentID: "x", N: 1}}
	cmd := m.Update(events.SessionsLoaded{Err: os.ErrPermission, LocalOnly: true})
	if cmd != nil || len(m.sessions) != 1 || len(m.events) != 1 || m.loading {
		t.Fatal("failed session inventory scheduled replacement of known usage")
	}
	if cmd := m.Update(events.SessionsLoaded{LocalOnly: true}); cmd == nil || len(m.sessions) != 0 {
		t.Fatal("successful empty inventory did not schedule a fresh aggregate")
	}
}
