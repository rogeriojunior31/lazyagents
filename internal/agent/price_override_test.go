package agent

import (
	"math"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func rateNumber(v float64) *float64 { return &v }
func testFallbackRate() PriceOverride {
	return PriceOverride{Agent: "gemini-cli", Model: "private-model", Billing: "api", Currency: "USD", Source: "https://pricing.example.test/rates", Date: "2026-10-18", Input: rateNumber(2), Output: rateNumber(8), CacheRead: rateNumber(0.5)}
}
func TestFallbackPriceValidationAndExactMatching(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	r := testFallbackRate()
	if err := ValidatePriceOverrides([]PriceOverride{r}, now); err != nil {
		t.Fatal(err)
	}
	e := UsageEvent{AgentID: r.Agent, Model: r.Model, Usage: Usage{Input: 1000000, Output: 1000000, CacheRead: 1000000}}
	cost := ResolveCost(e, AuthUnknown, []PriceOverride{r})
	if !cost.Known || cost.Amount != 10.5 || cost.Origin.Kind != "estimated" || cost.Origin.Date != r.Date {
		t.Fatal(cost)
	}
	for _, change := range []func(*UsageEvent){func(e *UsageEvent) { e.AgentID = "other" }, func(e *UsageEvent) { e.Provider = "other" }, func(e *UsageEvent) { e.Model = "different" }, func(e *UsageEvent) { e.Usage.Tier = "priority" }, func(e *UsageEvent) { e.Usage.Incomplete = true }} {
		modified := e
		change(&modified)
		if ResolveCost(modified, AuthUnknown, []PriceOverride{r}).Known {
			t.Fatal("fallback matched another identity or incomplete usage")
		}
	}
	if ResolveCost(e, AuthSubscription, []PriceOverride{r}).Known {
		t.Fatal("estimated subscription billing")
	}
	e.Usage.CostKnown = true
	cost = ResolveCost(e, AuthUnknown, []PriceOverride{r})
	if !cost.Known || cost.Amount != 0 || cost.Origin.Kind != "recorded" {
		t.Fatal("zero native cost was replaced")
	}
	e.Usage.Cost = 3.25
	cost = ResolveCost(e, AuthUnknown, []PriceOverride{r})
	if cost.Amount != 3.25 {
		t.Fatal("native cost was replaced")
	}
	e.Usage.CostKnown = false
	e.Usage.Cost = 0
	e.Usage.CacheWrite = 1
	if ResolveCost(e, AuthUnknown, []PriceOverride{r}).Known {
		t.Fatal("omitted cache rate assumed free")
	}
	r.CacheWrite = rateNumber(0)
	if !ResolveCost(e, AuthUnknown, []PriceOverride{r}).Known {
		t.Fatal("explicit free cache rate rejected")
	}
}
func TestInvalidFallbackPricesNeverQuoteValues(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	for _, edit := range []func(*PriceOverride){func(r *PriceOverride) { r.Input = nil }, func(r *PriceOverride) { r.Input = rateNumber(math.NaN()) }, func(r *PriceOverride) { r.Output = rateNumber(math.Inf(1)) }, func(r *PriceOverride) { r.CacheRead = rateNumber(-1) }, func(r *PriceOverride) { r.Currency = "BRL" }, func(r *PriceOverride) { r.Billing = "subscription" }, func(r *PriceOverride) { r.Date = "2026-10-19" }, func(r *PriceOverride) { r.Source = "https://user:secret-value@example.test/rates" }, func(r *PriceOverride) { r.Source = "https://example.test/?key=secret-value" }, func(r *PriceOverride) { r.Model = "secret-value\n" }} {
		r := testFallbackRate()
		edit(&r)
		err := ValidatePriceOverrides([]PriceOverride{r}, now)
		if err == nil || strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("validation: %v", err)
		}
	}
	r := testFallbackRate()
	if ValidatePriceOverrides([]PriceOverride{r, r}, now) == nil {
		t.Fatal("accepted duplicate rate")
	}
	for _, raw := range []string{"agent: x\ninput: 1\ninput: 2", "agent: x\nunknown: secret-value", "input: secret-value"} {
		var r PriceOverride
		err := yaml.Unmarshal([]byte(raw), &r)
		if err == nil || strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("yaml validation: %v", err)
		}
	}
}

func TestFallbackDoesNotReplaceKnownListPriceOrSyntheticZero(t *testing.T) {
	r := testFallbackRate()
	r.Agent = "claude-code"
	r.Model = "claude-sonnet-4"
	r.Input = rateNumber(999)
	e := UsageEvent{AgentID: r.Agent, Model: r.Model, Usage: Usage{Input: 1_000_000}}
	cost := ResolveCost(e, AuthAPIKey, []PriceOverride{r})
	if !cost.Known || cost.Amount != 3 || cost.Origin.Source == r.Source {
		t.Fatalf("known price replaced: %+v", cost)
	}
	e.Model = "<synthetic>"
	e.Usage = Usage{}
	if cost := ResolveCost(e, AuthAPIKey, nil); !cost.Known || cost.Amount != 0 {
		t.Fatal("synthetic errors blanked known-cost rows")
	}
}
