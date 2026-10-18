package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
)

func testRate() agent.PriceOverride {
	input, output, cache := 2.0, 8.0, 0.5
	return agent.PriceOverride{Agent: "gemini-cli", Model: "private-model", Billing: "api", Currency: "USD", Source: "https://pricing.example.test/rates", Date: "2026-10-18", Input: &input, Output: &output, CacheRead: &cache}
}
func TestUsageReportsPriceSourcesAndUnavailableCounters(t *testing.T) {
	rate := testRate()
	recorded := agent.UsageEvent{AgentID: "opencode", Provider: "actual-provider", Model: "private-model", Usage: agent.Usage{Input: 1_000_000, CostKnown: true, Cost: 3}, Time: time.Now()}
	estimated := agent.UsageEvent{AgentID: "gemini-cli", Model: "private-model", Usage: agent.Usage{Input: 1_000_000, Output: 1_000_000}, Time: time.Now()}
	price := pricerFor(map[string]bool{"opencode": true, "gemini-cli": true}, rate)
	total := Sum([]agent.UsageEvent{recorded, estimated}, price)
	j := toJSON(total, true)
	if total.Recorded != 1 || total.Estimated != 1 || j.CostKind != "mixed" || j.CostUSD == nil || *j.CostUSD != 13 || len(j.Sources) != 2 || j.Sources[0].Provider != "actual-provider" || !j.TokensComplete {
		t.Fatalf("evidence: %+v", j)
	}
	if money(total) != "~$13.00" {
		t.Fatal("estimate was not marked")
	}
	missing := estimated
	missing.Usage.Incomplete = true
	total = Sum([]agent.UsageEvent{recorded, estimated, missing}, price)
	j = toJSON(total, true)
	if j.CostUSD != nil || j.CostKind != "unavailable" || j.Unpriced != 1 || j.MissingUsage != 1 || j.TokensComplete {
		t.Fatalf("partial data presented as complete: %+v", j)
	}
	// The same model from another native provider is a separate row and has no matching fallback.
	other := estimated
	other.Provider = "another-provider"
	rows := ByModel([]agent.UsageEvent{estimated, other}, price)
	if len(rows) != 2 || rows[0].Priced == rows[1].Priced {
		t.Fatal("merged providers or used another provider's rate")
	}
}

type brokenUsage struct{ fakeAdapter }

func (b brokenUsage) UsageEvents(agent.Session) ([]agent.UsageEvent, error) {
	return nil, errors.New("private parser input")
}
func TestHistoryEvidenceReportsReadErrorsAndIgnoresCopiedMtime(t *testing.T) {
	calls := 0
	now := time.Now()
	good := fakeAdapter{id: "good", calls: &calls, events: []agent.UsageEvent{{Time: now, Model: "m", Usage: agent.Usage{Input: 10}}}, sessions: []agent.Session{{AgentID: "good", MTime: now.AddDate(-1, 0, 0)}}}
	broken := brokenUsage{fakeAdapter{id: "broken", calls: &calls, sessions: []agent.Session{{AgentID: "broken"}}}}
	empty := fakeAdapter{id: "empty", calls: &calls, sessions: []agent.Session{{AgentID: "empty"}}}
	svc := New([]agent.Adapter{good, broken, empty}, core.PathsIn(t.TempDir()))
	events, h := svc.recentWithEvidence(now.Add(-time.Minute), "")
	if len(events) != 1 || h.Unavailable != 1 || h.WithoutUsage != 1 || h.Sessions != 3 || h.ReadAt.IsZero() || !h.LastResponse.Equal(now) || calls != 0 {
		t.Fatalf("history: %+v %+v", events, h)
	}
	data, _ := json.Marshal(h)
	if bytes.Contains(data, []byte("private parser input")) {
		t.Fatal("history leaked raw errors")
	}
	if !strings.Contains(historyText(h), "observed responses only") {
		t.Fatal("partial scope hidden")
	}
}
func TestPriceConfigIsSharedByCLIAndTabAndNeverRewritesInputs(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `usage:
  prices:
    - agent: gemini-cli
      model: private-model
      billing: api
      currency: USD
      source: https://pricing.example.test/rates
      date: "2026-10-18"
      input: 2
      output: 8
`
	if err := os.WriteFile(paths.ConfigPath(), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := core.ReadConfig(paths.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	deps := &feature.Deps{Paths: paths, Config: cfg}
	f := Feature()
	_ = f.Commands(deps)
	tab := f.Tabs(deps)[0].(*Tab)
	if len(tab.svc.Prices) != 1 || tab.svc.PriceError != "" {
		t.Fatal("fallback config did not reach the shared service")
	}
	after, _ := os.ReadFile(paths.ConfigPath())
	if string(after) != config {
		t.Fatal("reading prices rewrote config")
	}
	// An invalid set is rejected as a whole, retaining token reporting and safe diagnostics.
	bad := strings.Replace(config, "input: 2", "input: private-invalid-value", 1)
	if err := os.WriteFile(paths.ConfigPath(), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = core.ReadConfig(paths.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	deps = &feature.Deps{Paths: paths, Config: cfg}
	f = Feature()
	_ = f.Commands(deps)
	tab = f.Tabs(deps)[0].(*Tab)
	if len(tab.svc.Prices) != 0 || tab.svc.PriceError == "" {
		t.Fatal("accepted invalid fallback set")
	}
	for _, notice := range deps.Notices() {
		if strings.Contains(notice, "private-invalid-value") {
			t.Fatal("invalid price value leaked")
		}
	}
	if _, err := os.Stat(filepath.Join(paths.Home, ".gemini")); !os.IsNotExist(err) {
		t.Fatal("price configuration touched native files")
	}
}
func TestNativeRecordedCostsWinWhenRatesChange(t *testing.T) {
	rate := testRate()
	event := agent.UsageEvent{AgentID: "gemini-cli", Model: rate.Model, Usage: agent.Usage{Input: 1_000_000}}
	old := Sum([]agent.UsageEvent{event}, pricerFor(map[string]bool{"gemini-cli": true}, rate))
	newInput := 5.0
	rate.Input = &newInput
	current := Sum([]agent.UsageEvent{event}, pricerFor(map[string]bool{"gemini-cli": true}, rate))
	if old.Cost != 2 || current.Cost != 5 || event.Usage.Cost != 0 {
		t.Fatal("rate edits altered native event or failed to reprice")
	}
	event.Usage.CostKnown = true
	if current := Sum([]agent.UsageEvent{event}, pricerFor(map[string]bool{"gemini-cli": true}, rate)); !current.Priced || current.Cost != 0 || current.Recorded != 1 {
		t.Fatal("zero native cost was replaced")
	}
}

func TestSmallPositiveCostsDoNotReadAsFree(t *testing.T) {
	t1 := Total{Events: 1, Priced: true, Cost: 0.000608, Estimated: 1}
	if money(t1) != "~$0.000608" {
		t.Fatal(money(t1))
	}
	t1.Estimated = 0
	t1.Recorded = 1
	t1.Cost = 0
	if money(t1) != "$0.00" {
		t.Fatal("native free cost changed")
	}
	t1.Cost = 0.0000001
	if money(t1) != "<$0.000001" {
		t.Fatal("tiny positive cost displayed as zero")
	}
}
