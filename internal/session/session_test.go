package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
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
	}, core.PathsIn(t.TempDir()))
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
	}, core.PathsIn(t.TempDir()))

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
	}, core.PathsIn(t.TempDir()))

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

// transcriptAdapter estende fakeAdapter com transcripts fixos por sessão,
// para testar SearchTranscripts sem depender de arquivos reais.
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
		{AgentID: "a", ID: "s1", Title: "sessão recente"},
		{AgentID: "a", ID: "s2", Title: "sessão antiga"},
		{AgentID: "a", ID: "s3", Title: "sessão quebrada"},
		{AgentID: "a", ID: "s4", Title: "sessão sem match"},
	}
	adapters := []agent.Adapter{
		transcriptAdapter{
			fakeAdapter: fakeAdapter{id: "a", sessions: sessions},
			transcripts: map[string][]agent.Entry{
				"s1": {{Role: "user", Text: "como configuro o Docker aqui?"}},
				"s2": {{Role: "assistant", Text: "texto bem longo antes... a resposta sobre DOCKER está aqui... e continua depois com mais contexto irrelevante para preencher"}},
				"s4": {{Role: "user", Text: "nada a ver com o assunto buscado"}},
			},
			transcriptErr: map[string]error{
				"s3": errors.New("transcript corrompido"),
			},
		},
	}

	matches, err := SearchTranscripts(adapters, sessions, "docker")
	if err == nil {
		t.Fatal("erro de transcript individual (s3) deveria ser propagado agregado")
	}
	if len(matches) != 2 {
		t.Fatalf("esperava 2 matches (s1, s2), veio %d: %+v", len(matches), matches)
	}
	ids := map[string]string{}
	for _, m := range matches {
		ids[m.Session.ID] = m.Excerpt
	}
	if _, ok := ids["s1"]; !ok {
		t.Error("s1 deveria casar (case-insensitive)")
	}
	if excerpt, ok := ids["s2"]; !ok || !strings.Contains(strings.ToLower(excerpt), "docker") {
		t.Errorf("s2 deveria casar com excerto contendo docker: %q", excerpt)
	}

	// query vazia: sem resultado, sem erro
	if m, err := SearchTranscripts(adapters, sessions, "   "); m != nil || err != nil {
		t.Errorf("query vazia deveria devolver nil, nil: %v %v", m, err)
	}

	// sem nenhum match
	if m, err := SearchTranscripts(adapters, []agent.Session{sessions[3]}, "docker"); len(m) != 0 || err != nil {
		t.Errorf("sem match deveria devolver vazio sem erro: %v %v", m, err)
	}
}

func TestResumeCmdRouting(t *testing.T) {
	svc := New([]agent.Adapter{
		fakeAdapter{id: "a"},
		fakeAdapter{id: "b"},
	}, core.PathsIn(t.TempDir()))
	argv, dir, ok := svc.ResumeCmd(agent.Session{AgentID: "b", ID: "s1"})
	if !ok || argv[0] != "b" || dir != "/dir/b" {
		t.Errorf("resume roteado errado: %v %s %v", argv, dir, ok)
	}
	if _, _, ok := svc.ResumeCmd(agent.Session{AgentID: "zzz"}); ok {
		t.Error("agente desconhecido deveria retornar ok=false")
	}
}
