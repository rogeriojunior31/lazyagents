package skills

import (
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestDetectionPreservesSkillAndAgentAndRejectsOldScan(t *testing.T) {
	m := inventoryTab(t)
	m.list.Select(1)
	selected, _ := m.selection()
	m.targets = []agent.Agent{{ID: "a"}, {ID: "b"}}
	m.col = 1
	m.Update(events.AgentsDetected{Revision: 1, Agents: []agent.Agent{{ID: "a", Installed: true, ManagedDir: "/a"}, {ID: "b", Installed: true, ManagedDir: "/b"}}})
	old := m.scanRequest
	m.Update(events.AgentsDetected{Revision: 2, Agents: []agent.Agent{{ID: "new", Installed: true, ManagedDir: "/new"}, {ID: "a", Installed: true, ManagedDir: "/a"}, {ID: "b", Installed: true, ManagedDir: "/b"}}})
	m.Update(scannedMsg{request: old, skills: []Skill{{Dir: "stale"}}})
	if len(m.skills) == 1 || m.targets[m.col].ID != "b" {
		t.Fatal("stale scan replaced inventory or agent selection moved")
	}
	m.Update(scannedMsg{request: m.scanRequest, skills: append([]Skill{{Dir: "aaa", Name: "aaa", Valid: true, InLibrary: true}}, m.skills...), issues: m.issues})
	if current, _ := m.selection(); current != selected {
		t.Fatalf("selected skill moved: %s -> %s", selected, current)
	}
}
