package tui

import (
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

func TestDetectionUsesSharedCallbackAndRejectsStaleBroadcasts(t *testing.T) {
	visible, hidden := &fakeMod{id: "visible"}, &fakeMod{id: "hidden"}
	calls := 0
	m := New([]module.Module{visible}, nil, "test", Options{NoSplash: true, Background: []module.Module{hidden}, DetectAgents: func(refresh bool) agent.Detection {
		if refresh {
			t.Fatal("boot requested a refresh")
		}
		calls++
		return agent.Detection{Revision: 1, Agents: []agent.Agent{{ID: "old"}}}
	}})
	boot := m.detectCmd()()
	fresh := events.AgentsDetected{Revision: 2, Agents: []agent.Agent{{ID: "new", Installed: true}}}
	model, _ := m.Update(fresh)
	m = model.(Model)
	for _, message := range []any{boot, events.AgentsDetected{Revision: 2}, events.SkillsScanned{Revision: 1}, events.SessionsLoaded{Revision: 1}} {
		model, _ = m.Update(message)
		m = model.(Model)
	}
	if calls != 1 || len(visible.got) != 1 || len(hidden.got) != 1 {
		t.Fatalf("stale result reached modules or boot duplicated scans: %d %v %v", calls, visible.got, hidden.got)
	}
	model, _ = m.Update(events.SkillsScanned{Revision: 2, Total: 3})
	m = model.(Model)
	m.Update(events.SessionsLoaded{Revision: 2})
	if len(visible.got) != 3 || len(hidden.got) != 3 {
		t.Fatal("fresh aggregates did not reach visible and hidden modules")
	}
}

func TestSessionBroadcastKeepsNewestLoadAcrossAllTabs(t *testing.T) {
	visible, hidden := &fakeMod{id: "visible"}, &fakeMod{id: "hidden"}
	m := New([]module.Module{visible}, nil, "test", Options{NoSplash: true, Background: []module.Module{hidden}})
	for _, message := range []events.SessionsLoaded{
		{Request: 2, Sessions: []agent.Session{{ID: "fresh"}}},
		{Request: 1, Sessions: []agent.Session{{ID: "obsolete"}}},
		{Request: 2},
	} {
		model, _ := m.Update(message)
		m = model.(Model)
	}
	if len(visible.got) != 1 || len(hidden.got) != 1 || visible.got[0].(events.SessionsLoaded).Sessions[0].ID != "fresh" {
		t.Fatal("a late session load reached visible or hidden consumers")
	}
}
