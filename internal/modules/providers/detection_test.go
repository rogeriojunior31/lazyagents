package providers

import (
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestDetectionReloadKeepsProviderAndAgentIdentity(t *testing.T) {
	a, b := &fakeHost{id: "a", installed: true}, &fakeHost{id: "b", installed: true}
	svc := New([]agent.Adapter{a, b}, core.PathsIn(t.TempDir()))
	for _, name := range []string{"middle", "last"} {
		if err := svc.Save(agent.ProviderProfile{Name: name, Model: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	m := newTab(svc)
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	m.cursor, m.col = 1, 1
	stale := m.loadCmd()()
	if err := svc.Save(agent.ProviderProfile{Name: "aaa", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	svc.adapters = []agent.Adapter{b, a}
	latest := m.Update(events.AgentsDetected{Revision: 2})
	m.Update(stale)
	if len(m.profiles) != 2 {
		t.Fatal("stale result was applied")
	}
	run(t, &m, latest)
	profile, _ := m.current()
	if profile.Name != "middle" || m.col != 0 || m.statuses[0].AgentID != "b" || len(m.profiles) != 3 {
		t.Fatalf("identity changed after refresh: %+v", m)
	}
}
