package usage

import (
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
