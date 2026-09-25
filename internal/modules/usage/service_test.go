package usage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func at(h int, m int) time.Time { return time.Date(2026, 9, 22, h, m, 0, 0, time.UTC) }

func ev(t time.Time, cwd string, in, out int) agent.UsageEvent {
	return agent.UsageEvent{Time: t, CWD: cwd, Model: "claude-opus-4", Usage: agent.Usage{Input: in, Output: out, Model: "claude-opus-4"}}
}

func TestBlocks(t *testing.T) {
	cases := []struct {
		name   string
		events []agent.UsageEvent
		now    time.Time
		want   []struct {
			start  time.Time
			events int
			active bool
		}
	}{
		{
			name:   "dentro da janela de 5h",
			events: []agent.UsageEvent{ev(at(9, 0), "/p", 1, 1), ev(at(13, 59), "/p", 1, 1)},
			now:    at(13, 59), // 14:00 já seria o fim exato da janela
			want: []struct {
				start  time.Time
				events int
				active bool
			}{{at(9, 0), 2, true}},
		},
		{
			name:   "gap maior que a janela abre outro bloco",
			events: []agent.UsageEvent{ev(at(9, 0), "/p", 1, 1), ev(at(14, 0), "/p", 1, 1)},
			now:    at(15, 0),
			want: []struct {
				start  time.Time
				events int
				active bool
			}{{at(9, 0), 1, false}, {at(14, 0), 1, true}},
		},
		{
			name:   "bloco antigo não fica ativo",
			events: []agent.UsageEvent{ev(at(1, 0), "/p", 1, 1)},
			now:    at(23, 0),
			want: []struct {
				start  time.Time
				events int
				active bool
			}{{at(1, 0), 1, false}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Blocks(tc.events, tc.now)
			if len(got) != len(tc.want) {
				t.Fatalf("%d blocos, quer %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				b := got[i]
				if !b.Start.Equal(w.start) || b.Events != w.events || b.Active != w.active {
					t.Errorf("bloco %d = %+v, quer start %s events %d active %v", i, b, w.start, w.events, w.active)
				}
				if !b.End.Equal(b.Start.Add(BlockWindow)) {
					t.Errorf("bloco %d: fim = %s", i, b.End)
				}
			}
			cur, ok := Current(got)
			wantCur := len(tc.want) > 0 && tc.want[len(tc.want)-1].active
			if ok != wantCur || (ok && !cur.Active) {
				t.Errorf("Current = %+v %v, quer ativo=%v", cur, ok, wantCur)
			}
		})
	}
	if got := Blocks(nil, time.Now()); got != nil {
		t.Errorf("sem eventos = %+v", got)
	}
}

func TestDailyAndByProject(t *testing.T) {
	events := []agent.UsageEvent{
		ev(at(9, 0), "/home/u/alpha", 10, 1),
		ev(at(10, 0), "/home/u/beta", 5, 1),
		ev(at(9, 0).AddDate(0, 0, -1), "/home/u/alpha", 1, 1),
		ev(at(9, 0).AddDate(0, 0, -2), "", 1, 1),
	}
	days := Daily(events, 2, nil)
	if len(days) != 2 || days[0].Label != "2026-09-22" || days[0].Events != 2 || days[0].Tokens != 17 {
		t.Fatalf("Daily = %+v", days)
	}
	if days[1].Label != "2026-09-21" {
		t.Errorf("ordem decrescente: %+v", days)
	}
	projs := ByProject(events, nil)
	if len(projs) != 3 || projs[0].Label != "alpha" || projs[0].Tokens != 13 {
		t.Fatalf("ByProject = %+v", projs)
	}
	if projs[2].Label != "no project" {
		t.Errorf("CWD vazio deve virar 'sem projeto': %+v", projs)
	}
}

func TestCostOnlyForAPIKey(t *testing.T) {
	u := agent.Usage{Input: 1_000_000, Output: 0, Model: "claude-opus-4"}
	if _, ok := Cost(u, agent.AuthSubscription); ok {
		t.Error("assinatura não deve estimar custo por token")
	}
	c, ok := Cost(u, agent.AuthAPIKey)
	if !ok || c < 14.9 || c > 15.1 {
		t.Errorf("custo por API key = %v %v", c, ok)
	}
	if _, ok := Cost(agent.Usage{Model: "gpt-6-astra", Input: 10}, agent.AuthAPIKey); ok {
		t.Error("modelo fora da tabela não deve estimar custo")
	}
}

// fakeAdapter mínimo com as capacidades de uso.
type fakeAdapter struct {
	id       string
	auth     agent.AuthMode
	status   agent.RateStatus
	err      error
	calls    *int
	events   []agent.UsageEvent
	sessions []agent.Session
}

func (f fakeAdapter) ID() string                                       { return f.id }
func (f fakeAdapter) Detect() agent.Agent                              { return agent.Agent{ID: f.id} }
func (f fakeAdapter) ListSessions() ([]agent.Session, error)           { return f.sessions, nil }
func (f fakeAdapter) ResumeCmd(agent.Session) ([]string, string, bool) { return nil, "", false }
func (f fakeAdapter) Transcript(agent.Session) ([]agent.Entry, error)  { return nil, nil }
func (f fakeAdapter) DeleteSession(agent.Session, string) error        { return nil }
func (f fakeAdapter) AuthMode() (agent.AuthMode, string)               { return f.auth, "detalhe" }
func (f fakeAdapter) RateLimits(context.Context) (agent.RateStatus, error) {
	*f.calls++
	return f.status, f.err
}
func (f fakeAdapter) UsageEvents(agent.Session) ([]agent.UsageEvent, error) { return f.events, nil }

func TestStatusCache(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	calls := 0
	ad := fakeAdapter{id: "x", auth: agent.AuthSubscription, calls: &calls, status: agent.RateStatus{
		Plan: "max", FetchedAt: time.Now(), Source: "api",
		Windows: []agent.RateWindow{{Kind: agent.WindowSession, Label: "session 5h", UsedPercent: 12}},
	}}
	svc := New([]agent.Adapter{ad}, paths)

	st := svc.Status(context.Background(), false)
	if len(st) != 1 || st[0].Limits.Plan != "max" || st[0].Cached || calls != 1 {
		t.Fatalf("1ª chamada = %+v (calls=%d)", st, calls)
	}
	if st[0].AuthLabel != "assinatura" || st[0].AuthDetail != "detalhe" {
		t.Errorf("auth = %+v", st[0])
	}
	// service novo: o cache em disco evita a rede
	st = New([]agent.Adapter{ad}, paths).Status(context.Background(), false)
	if calls != 1 || !st[0].Cached || st[0].Limits.Plan != "max" {
		t.Fatalf("cache não usado: %+v (calls=%d)", st, calls)
	}
	// refresh força
	if st = svc.Status(context.Background(), true); calls != 2 || st[0].Cached {
		t.Fatalf("refresh não forçou: %+v (calls=%d)", st, calls)
	}
	// TTL vencido força
	svc.TTL = time.Nanosecond
	if svc.Status(context.Background(), false); calls != 3 {
		t.Fatalf("TTL não respeitado (calls=%d)", calls)
	}
}

func TestStatusErrorKeepsStaleAndOtherAgents(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	calls := 0
	ok := fakeAdapter{id: "ok", auth: agent.AuthAPIKey, calls: &calls, status: agent.RateStatus{Plan: "p", FetchedAt: time.Now()}}
	bad := fakeAdapter{id: "bad", calls: &calls, err: os.ErrDeadlineExceeded}
	svc := New([]agent.Adapter{ok, bad}, paths)
	st := svc.Status(context.Background(), false)
	if len(st) != 2 || st[0].Err != "" || st[1].Err == "" {
		t.Fatalf("status = %+v", st)
	}
	// cache corrompido não quebra
	if err := os.WriteFile(paths.UsageCachePath(), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st = svc.Status(context.Background(), false); len(st) != 2 {
		t.Fatalf("cache corrompido derrubou o status: %+v", st)
	}
	if !strings.Contains(st[1].Err, "timeout") {
		t.Errorf("erro do agente = %q", st[1].Err)
	}
}

func TestEventsAggregatesAndTagsAgent(t *testing.T) {
	events := []agent.UsageEvent{ev(at(10, 0), "/p", 1, 1), ev(at(9, 0), "/p", 2, 2)}
	calls := 0
	svc := New([]agent.Adapter{fakeAdapter{id: "x", calls: &calls, events: events}}, core.PathsIn(t.TempDir()))
	got := svc.Events([]agent.Session{{AgentID: "x", ID: "1"}})
	if len(got) != 2 || !got[0].Time.Equal(at(9, 0)) {
		t.Fatalf("eventos devem sair ordenados: %+v", got)
	}
	if got[0].AgentID != "x" {
		t.Errorf("AgentID não preenchido: %+v", got[0])
	}
	if n := len(svc.Events([]agent.Session{{AgentID: "inexistente"}})); n != 0 {
		t.Errorf("agente desconhecido = %d eventos", n)
	}
}

func TestNullCacheAndFutureTimestamp(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	calls := 0
	ad := fakeAdapter{id: "x", calls: &calls, status: agent.RateStatus{FetchedAt: time.Now()}}
	svc := New([]agent.Adapter{ad}, paths)
	svc.Status(context.Background(), false)
	if err := os.WriteFile(paths.UsageCachePath(), []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := svc.Status(context.Background(), false); len(got) != 1 || calls != 2 {
		t.Fatalf("%v calls=%d", got, calls)
	}
	if fresh(cacheEntry{FetchedAt: time.Now().Add(time.Hour)}, time.Now(), DefaultTTL) {
		t.Fatal("future cache accepted")
	}
}

func TestAggregationSeparatesPathsAndMixedModels(t *testing.T) {
	a, b := ev(at(9, 0), "/one/app", 10, 0), ev(at(10, 0), "/two/app", 20, 0)
	if got := ByProject([]agent.UsageEvent{a, b}, nil); len(got) != 2 {
		t.Fatalf("merged projects: %+v", got)
	}
	b.Usage.Model = "claude-sonnet-4"
	blocks := Blocks([]agent.UsageEvent{a, b}, at(11, 0))
	if _, ok := Cost(blocks[0].Usage, agent.AuthAPIKey); ok {
		t.Fatal("priced mixed models using one rate")
	}
}

// A cache written before v2 holds Portuguese window labels: it must be
// dropped, even as the stale fallback when the agent fails.
func TestStatusDropsOutdatedCache(t *testing.T) {
	legacy := `{"x": {"plan": "max", "windows": [{"kind": "session", "label": "sessão 5h", "used_percent": 12}], "fetched_at": "` + // check-english:allow
		time.Now().Format(time.RFC3339) + `"}}`
	fresh := agent.RateStatus{Plan: "max", FetchedAt: time.Now(),
		Windows: []agent.RateWindow{{Kind: agent.WindowSession, Label: "session 5h", UsedPercent: 12}}}

	writeLegacy := func(t *testing.T) core.Paths {
		t.Helper()
		paths := core.PathsIn(t.TempDir())
		if err := os.MkdirAll(filepath.Dir(paths.UsageCachePath()), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.UsageCachePath(), []byte(legacy), 0o644); err != nil {
			t.Fatal(err)
		}
		return paths
	}

	t.Run("fresh legacy cache is refetched", func(t *testing.T) {
		paths := writeLegacy(t)
		calls := 0
		ad := fakeAdapter{id: "x", calls: &calls, status: fresh}
		st := New([]agent.Adapter{ad}, paths).Status(context.Background(), false)
		if calls != 1 || st[0].Cached || st[0].Limits.Windows[0].Label != "session 5h" {
			t.Fatalf("legacy cache used: %+v (calls=%d)", st, calls)
		}
		// and the rewritten cache is current
		st = New([]agent.Adapter{ad}, paths).Status(context.Background(), false)
		if calls != 1 || !st[0].Cached {
			t.Fatalf("new cache not used: %+v (calls=%d)", st, calls)
		}
	})
	t.Run("legacy cache is not the stale fallback", func(t *testing.T) {
		paths := writeLegacy(t)
		calls := 0
		bad := fakeAdapter{id: "x", calls: &calls, err: os.ErrDeadlineExceeded}
		st := New([]agent.Adapter{bad}, paths).Status(context.Background(), false)
		if st[0].Cached || len(st[0].Limits.Windows) != 0 || st[0].Err == "" {
			t.Fatalf("legacy cache shown: %+v", st)
		}
	})
}
