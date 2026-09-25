package usage

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 9, 23, 15, 30, 0, 0, time.Local)
	cases := []struct {
		in    string
		want  time.Time
		label string
	}{
		{"7d", time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local), "last 7 days"},
		{"1d", time.Date(2026, 9, 23, 0, 0, 0, 0, time.Local), "today"},
		{"24h", now.Add(-24 * time.Hour), "last 24 hours"},
		{"2026-09-01", time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local), "since 2026-09-01"},
	}
	for _, tc := range cases {
		got, label, err := parseSince(tc.in, now)
		if err != nil || !got.Equal(tc.want) || label != tc.label {
			t.Errorf("parseSince(%q) = %s %q %v, want %s %q", tc.in, got, label, err, tc.want, tc.label)
		}
	}
	for _, bad := range []string{"", "d", "0d", "-3d", "7dh", "7x", "yesterday"} {
		if _, _, err := parseSince(bad, now); err == nil {
			t.Errorf("parseSince(%q) accepted", bad)
		}
	}
}

func TestFillDaysIsContinuous(t *testing.T) {
	from := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.Local)
	got := fillDays([]Total{{Label: "2026-09-21", Tokens: 5}}, from, now)
	if len(got) != 4 || got[0].Label != "2026-09-20" || got[1].Tokens != 5 || got[3].Label != "2026-09-23" {
		t.Fatalf("fillDays = %+v", got)
	}
}

// Cost is summed per event: each model has its own rate, and a subscription
// agent's event leaves the aggregate unpriced.
func TestAggregatesPricePerEvent(t *testing.T) {
	opus := ev(at(9, 0), "/p", 1_000_000, 0) // claude-opus-4: $15/MTok input
	opus.AgentID = "api"
	sonnet := ev(at(10, 0), "/p", 1_000_000, 0)
	sonnet.AgentID, sonnet.Model, sonnet.Usage.Model = "api", "claude-sonnet-4", "claude-sonnet-4" // $3
	sub := ev(at(11, 0), "/q", 10, 0)
	sub.AgentID = "sub"
	price := pricerFor(map[string]bool{"api": true})

	agents := ByAgent([]agent.UsageEvent{opus, sonnet, sub}, price)
	if len(agents) != 2 || agents[0].Label != "api" || !agents[0].Priced || agents[0].Cost < 17.99 || agents[0].Cost > 18.01 {
		t.Fatalf("ByAgent = %+v", agents)
	}
	if agents[1].Priced {
		t.Errorf("subscription must have no cost: %+v", agents[1])
	}
	if total := Sum([]agent.UsageEvent{opus, sonnet, sub}, price); total.Priced || total.Tokens != 2_000_010 {
		t.Errorf("Sum with an unpriced event = %+v", total)
	}
	models := ByModel([]agent.UsageEvent{opus, sonnet}, price)
	if len(models) != 2 || models[0].Label != "claude-opus-4" {
		t.Errorf("ByModel = %+v", models)
	}
	if Sum(nil, nil).Priced {
		t.Error("no pricer, no cost")
	}
}

func TestTableAlignsIgnoringANSI(t *testing.T) {
	styled := lipgloss.NewStyle().Bold(true).Render("ab")
	out := table([]string{"X", "N"}, [][]string{{styled, "1"}, {"abcd", "100"}}, nil)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if w0, w1 := lipgloss.Width(lines[1]), lipgloss.Width(lines[2]); w0 != w1 {
		t.Errorf("misaligned rows (%d≠%d):\n%s", w0, w1, out)
	}
}

// runUsage runs the command against a fake agent with a recent session.
func runUsage(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	now := time.Now()
	e1 := agent.UsageEvent{Time: now.Add(-time.Hour), CWD: "/home/u/alpha", Model: "claude-opus-4", Usage: agent.Usage{Input: 100, Output: 50}}
	e2 := agent.UsageEvent{Time: now.Add(-30 * time.Minute), CWD: "/home/u/beta", Model: "claude-opus-4", Usage: agent.Usage{Input: 10, Output: 5}}
	calls := 0
	ad := fakeAdapter{id: "x", auth: agent.AuthSubscription, calls: &calls,
		status:   agent.RateStatus{Plan: "max", FetchedAt: now, Windows: []agent.RateWindow{{Label: "session 5h", UsedPercent: 42, ResetsAt: now.Add(time.Hour)}}},
		events:   []agent.UsageEvent{e1, e2},
		sessions: []agent.Session{{AgentID: "x", ID: "1", MTime: now}}}
	svc := New([]agent.Adapter{ad}, core.PathsIn(t.TempDir()))
	var out, errOut bytes.Buffer
	code := cmdUsage(args, cli.Context{Out: &out, Err: &errOut, AgentIDs: []string{"x"}}, svc)
	return out.String(), errOut.String(), code
}

func TestUsageViews(t *testing.T) {
	out, _, code := runUsage(t)
	for _, want := range []string{"session 5h", "42.0%", "Current block", "Last 7 days", "165 tokens", "alpha"} {
		if !strings.Contains(out, want) {
			t.Errorf("panel without %q (exit %d):\n%s", want, code, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("non-terminal output must have no ANSI")
	}
	out, _, _ = runUsage(t, "projects", "--limit", "1")
	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta ") || !strings.Contains(out, "1 more") {
		t.Errorf("projects --limit 1:\n%s", out)
	}
	out, _, code = runUsage(t, "agents", "--json")
	var rep jsonReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil || code != 0 {
		t.Fatalf("agents --json = %v (exit %d): %s", err, code, out)
	}
	if len(rep.Rows) != 1 || rep.Rows[0].Label != "x" || rep.Total.Tokens != 165 || rep.Total.CostUSD != nil {
		t.Errorf("report = %+v", rep)
	}
}

func TestUsageRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{{"--agent", "nope"}, {"weekly"}, {"--since", "yesterday"}, {"daily", "extra"}} {
		if _, errOut, code := runUsage(t, args...); code != 1 || errOut == "" {
			t.Errorf("%v = exit %d, stderr %q", args, code, errOut)
		}
	}
}
