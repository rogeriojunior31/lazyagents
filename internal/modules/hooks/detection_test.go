package hooks

import (
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestDetectionReloadKeepsHookIdentityAndRejectsOldLoad(t *testing.T) {
	svc := New(nil, core.PathsIn(t.TempDir()))
	for _, name := range []string{"middle", "last"} {
		if err := svc.Save(Hook{Name: name, Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "echo test"}}}); err != nil {
			t.Fatal(err)
		}
	}
	m := newTab(svc)
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	m.cursor = 1
	stale := m.loadCmd()()
	if err := svc.Save(Hook{Name: "aaa", Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "echo test"}}}); err != nil {
		t.Fatal(err)
	}
	latest := m.Update(events.AgentsDetected{Revision: 2})
	run(t, &m, latest)
	m.Update(stale)
	hook, _ := m.current()
	if hook.Name != "middle" || len(m.lib) != 3 {
		t.Fatalf("hook identity or fresh result lost: %+v", m.lib)
	}
}
