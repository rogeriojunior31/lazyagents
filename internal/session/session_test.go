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

// liveAdapter estende fakeAdapter implementando agent.LiveChecker.
type liveAdapter struct {
	fakeAdapter
	live map[string]bool
}

func (l liveAdapter) IsLive(s agent.Session) bool { return l.live[s.ID] }

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

// usageAdapter estende fakeAdapter implementando agent.UsageReader, para
// testar o type assertion opcional do SessionUsage.
type usageAdapter struct {
	fakeAdapter
	usage agent.Usage
	ok    bool
}

func (u usageAdapter) SessionUsage(agent.Session) (agent.Usage, bool) { return u.usage, u.ok }

func TestSessionUsageRouting(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "sem-usage"},
		usageAdapter{fakeAdapter: fakeAdapter{id: "com-usage"}, usage: agent.Usage{Input: 10}, ok: true},
	}, "")

	if _, ok := svc.SessionUsage(agent.Session{AgentID: "sem-usage"}); ok {
		t.Error("adapter sem UsageReader deveria devolver ok=false")
	}
	u, ok := svc.SessionUsage(agent.Session{AgentID: "com-usage"})
	if !ok || u.Input != 10 {
		t.Errorf("usage roteado errado: %+v %v", u, ok)
	}
	if _, ok := svc.SessionUsage(agent.Session{AgentID: "zzz"}); ok {
		t.Error("agente desconhecido deveria retornar ok=false")
	}
}

func TestIsLiveAndDeleteRefusal(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "sem-suporte"},
		liveAdapter{fakeAdapter: fakeAdapter{id: "com-suporte"}, live: map[string]bool{"viva": true}},
	}, "")

	if svc.IsLive(agent.Session{AgentID: "sem-suporte", ID: "x"}) {
		t.Error("adapter sem LiveChecker deveria ser sempre false")
	}
	if !svc.IsLive(agent.Session{AgentID: "com-suporte", ID: "viva"}) {
		t.Error("sessão marcada viva no adapter deveria ser true")
	}
	if svc.IsLive(agent.Session{AgentID: "com-suporte", ID: "morta"}) {
		t.Error("sessão não marcada deveria ser false")
	}

	// deletar uma sessão viva é recusado, sem chamar o DeleteSession do adapter
	if err := svc.DeleteSession(agent.Session{AgentID: "com-suporte", ID: "viva"}); err == nil {
		t.Fatal("deletar sessão viva deveria ser recusado")
	}
	// sessão não viva do mesmo adapter continua deletável normalmente
	if err := svc.DeleteSession(agent.Session{AgentID: "com-suporte", ID: "morta"}); err != nil {
		t.Fatalf("sessão não viva não deveria ser recusada: %v", err)
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
