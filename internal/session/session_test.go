package session

import (
	"errors"
	"testing"
	"time"

	"lazyskills/internal/agent"
)

// fakeAdapter implementa agent.Adapter em memória.
type fakeAdapter struct {
	id       string
	sessions []agent.Session
	err      error
}

func (f fakeAdapter) ID() string { return f.id }
func (f fakeAdapter) Detect() agent.Agent {
	return agent.Agent{ID: f.id, Name: f.id, Installed: true}
}
func (f fakeAdapter) ListSessions() ([]agent.Session, error) { return f.sessions, f.err }
func (f fakeAdapter) ResumeCmd(s agent.Session) ([]string, string, bool) {
	return []string{f.id, "resume", s.ID}, "/dir/" + f.id, true
}
func (f fakeAdapter) Transcript(agent.Session) ([]agent.Entry, error) {
	return []agent.Entry{{Role: "user", Text: "oi " + f.id}}, nil
}
func (f fakeAdapter) DeleteSession(agent.Session, string) error { return nil }

func TestListMergesAndSorts(t *testing.T) {
	t0 := time.Now()
	svc := New([]agent.Adapter{
		fakeAdapter{id: "a", sessions: []agent.Session{
			{AgentID: "a", ID: "a1", MTime: t0.Add(-2 * time.Hour)},
			{AgentID: "a", ID: "a2", MTime: t0},
		}},
		fakeAdapter{id: "b", sessions: []agent.Session{
			{AgentID: "b", ID: "b1", MTime: t0.Add(-time.Hour)},
		}},
		fakeAdapter{id: "c", err: errors.New("quebrou")},
	}, "")
	got, err := svc.List()
	if err == nil {
		t.Fatal("erro do adapter c deveria ser propagado agregado")
	}
	if len(got) != 3 {
		t.Fatalf("sessões = %d, quer 3 (falha de um agente não derruba os demais)", len(got))
	}
	wantOrder := []string{"a2", "b1", "a1"}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Errorf("posição %d = %s, quer %s", i, got[i].ID, want)
		}
	}
}

func TestResumeCmdRouting(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "a"},
		fakeAdapter{id: "b"},
	}, "")
	argv, dir, ok := svc.ResumeCmd(agent.Session{AgentID: "b", ID: "s1"})
	if !ok || argv[0] != "b" || dir != "/dir/b" {
		t.Errorf("resume roteado errado: %v %s %v", argv, dir, ok)
	}
	if _, _, ok := svc.ResumeCmd(agent.Session{AgentID: "zzz"}); ok {
		t.Error("agente desconhecido deveria retornar ok=false")
	}
}
