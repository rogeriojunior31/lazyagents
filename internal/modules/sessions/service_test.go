package sessions

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// fakeAdapter is an in-memory agent.Adapter.
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
	return []agent.Entry{{Role: "user", Text: "hi " + f.id}}, nil
}
func (f fakeAdapter) DeleteSession(agent.Session, string) error { return nil }

// liveAdapter adds agent.LiveChecker.
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
		fakeAdapter{id: "c", err: errors.New("broken")},
	}, core.PathsIn(t.TempDir()))
	got, err := svc.List()
	if err == nil {
		t.Fatal("adapter c error should be joined into the result")
	}
	if len(got) != 3 {
		t.Fatalf("sessions = %d, want 3 (one failing agent must not hide the others)", len(got))
	}
	wantOrder := []string{"a2", "b1", "a1"}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Errorf("position %d = %s, want %s", i, got[i].ID, want)
		}
	}
}

// usageAdapter adds agent.UsageReader.
type usageAdapter struct {
	fakeAdapter
	usage agent.Usage
	ok    bool
}

func (u usageAdapter) SessionUsage(agent.Session) (agent.Usage, bool) { return u.usage, u.ok }

func TestSessionUsageRouting(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "no-usage"},
		usageAdapter{fakeAdapter: fakeAdapter{id: "with-usage"}, usage: agent.Usage{Input: 10}, ok: true},
	}, core.PathsIn(t.TempDir()))

	if _, ok := svc.SessionUsage(agent.Session{AgentID: "no-usage"}); ok {
		t.Error("adapter without UsageReader should return ok=false")
	}
	u, ok := svc.SessionUsage(agent.Session{AgentID: "with-usage"})
	if !ok || u.Input != 10 {
		t.Errorf("usage routed wrong: %+v %v", u, ok)
	}
	if _, ok := svc.SessionUsage(agent.Session{AgentID: "zzz"}); ok {
		t.Error("unknown agent should return ok=false")
	}
}

func TestIsLiveAndDeleteRefusal(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "unsupported"},
		liveAdapter{fakeAdapter: fakeAdapter{id: "supported"}, live: map[string]bool{"live": true}},
	}, core.PathsIn(t.TempDir()))

	if svc.IsLive(agent.Session{AgentID: "unsupported", ID: "x"}) {
		t.Error("adapter without LiveChecker should always be false")
	}
	if !svc.IsLive(agent.Session{AgentID: "supported", ID: "live"}) {
		t.Error("session marked live should be true")
	}
	if svc.IsLive(agent.Session{AgentID: "supported", ID: "dead"}) {
		t.Error("unmarked session should be false")
	}

	// deleting a live session is refused without calling the adapter
	if err := svc.DeleteSession(agent.Session{AgentID: "supported", ID: "live"}); err == nil {
		t.Fatal("deleting a live session should be refused")
	}
	// a non-live session of the same adapter is still deletable
	if err := svc.DeleteSession(agent.Session{AgentID: "supported", ID: "dead"}); err != nil {
		t.Fatalf("non-live session should not be refused: %v", err)
	}
}

// transcriptAdapter serves fixed transcripts per session.
type transcriptAdapter struct {
	fakeAdapter
	transcripts   map[string][]agent.Entry
	transcriptErr map[string]error
}

func (t transcriptAdapter) Transcript(s agent.Session) ([]agent.Entry, error) {
	if err := t.transcriptErr[s.ID]; err != nil {
		return nil, err
	}
	return t.transcripts[s.ID], nil
}

func TestSearchTranscripts(t *testing.T) {
	sessions := []agent.Session{
		{AgentID: "a", ID: "s1", Title: "recent session"},
		{AgentID: "a", ID: "s2", Title: "old session"},
		{AgentID: "a", ID: "s3", Title: "broken session"},
		{AgentID: "a", ID: "s4", Title: "session without match"},
	}
	adapters := []agent.Adapter{
		transcriptAdapter{
			fakeAdapter: fakeAdapter{id: "a", sessions: sessions},
			transcripts: map[string][]agent.Entry{
				"s1": {{Role: "user", Text: "how do I set up Docker here?"}},
				"s2": {{Role: "assistant", Text: "a long text before... the answer about DOCKER is here... and it goes on after with more filler context to pad it"}},
				"s4": {{Role: "user", Text: "nothing to do with the query"}},
			},
			transcriptErr: map[string]error{
				"s3": errors.New("corrupt transcript"),
			},
		},
	}

	matches, err := SearchTranscripts(adapters, sessions, "docker")
	if err == nil {
		t.Fatal("the s3 transcript error should be joined into the result")
	}
	if len(matches) != 2 {
		t.Fatalf("want 2 matches (s1, s2), got %d: %+v", len(matches), matches)
	}
	ids := map[string]string{}
	for _, m := range matches {
		ids[m.Session.ID] = m.Excerpt
	}
	if _, ok := ids["s1"]; !ok {
		t.Error("s1 should match (case-insensitive)")
	}
	if excerpt, ok := ids["s2"]; !ok || !strings.Contains(strings.ToLower(excerpt), "docker") {
		t.Errorf("s2 should match with an excerpt containing docker: %q", excerpt)
	}

	// empty query: no result, no error
	if m, err := SearchTranscripts(adapters, sessions, "   "); m != nil || err != nil {
		t.Errorf("empty query should return nil, nil: %v %v", m, err)
	}

	// no match
	if m, err := SearchTranscripts(adapters, []agent.Session{sessions[3]}, "docker"); len(m) != 0 || err != nil {
		t.Errorf("no match should return empty without error: %v %v", m, err)
	}
}

func TestResumeCmdRouting(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "a"},
		fakeAdapter{id: "b"},
	}, core.PathsIn(t.TempDir()))
	argv, dir, ok := svc.ResumeCmd(agent.Session{AgentID: "b", ID: "s1"})
	if !ok || argv[0] != "b" || dir != "/dir/b" {
		t.Errorf("resume routed wrong: %v %s %v", argv, dir, ok)
	}
	if _, _, ok := svc.ResumeCmd(agent.Session{AgentID: "zzz"}); ok {
		t.Error("unknown agent should return ok=false")
	}
}
