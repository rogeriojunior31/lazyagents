package agent

import (
	"math"
	"testing"
)

func TestEstimateCost(t *testing.T) {
	u := Usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000, Model: "claude-sonnet-4-5-20250929"}
	cost, ok := EstimateCost(u)
	if !ok {
		t.Fatal("a known model should have a cost estimate")
	}
	want := 3.0 + 15.0 + 0.3 + 3.75
	if cost != want {
		t.Fatalf("cost = %v, want %v", cost, want)
	}

	if _, ok := EstimateCost(Usage{Model: "unknown-model-xyz", Input: 1}); ok {
		t.Fatal("an unknown model should give ok=false")
	}
	if _, ok := EstimateCost(Usage{Input: 1}); ok {
		t.Fatal("no model should give ok=false")
	}
	if c, ok := EstimateCost(Usage{Model: "<synthetic>"}); !ok || c != 0 {
		t.Fatal("a response with no tokens costs nothing, even with an unknown model")
	}
}

func TestVersionSpecificPricing(t *testing.T) {
	for _, tc := range []struct {
		model string
		input float64
		known bool
	}{
		{"claude-opus-4-5-20251101", 5, true}, {"claude-opus-4-1", 15, true},
		{"claude-opus-4-99", 0, false}, {"claude-opus-5", 5, true},
	} {
		got, ok := EstimateCost(Usage{Model: tc.model, Input: 1_000_000})
		if got != tc.input || ok != tc.known {
			t.Fatalf("%s: %v %v", tc.model, got, ok)
		}
	}
}

// The rules of the pricing page: 1-hour cache writes cost 2x input, fast mode
// has its own price on the models that offer it, batch halves everything, US
// inference costs 1.1x on Claude 4.6+; what the page does not price stays
// unknown.
func TestEstimateCostTiers(t *testing.T) {
	const M = 1_000_000
	for _, tc := range []struct {
		name string
		u    Usage
		want float64
		ok   bool
	}{
		{"1h cache writes", Usage{Model: "claude-sonnet-4-6", CacheWrite: 2 * M, CacheWrite1h: M}, 3.75 + 6, true},
		{"fast opus 5", Usage{Model: "claude-opus-5", Input: M, Output: M, CacheRead: M, Tier: "fast"}, 10 + 50 + 1, true},
		{"fast opus 5.5", Usage{Model: "claude-opus-5-5", Input: M, Tier: "fast"}, 8, true},
		{"fast without fast mode", Usage{Model: "claude-opus-4-7", Input: M, Tier: "fast"}, 0, false},
		{"batch", Usage{Model: "claude-haiku-4-5", Input: M, Output: M, Tier: "batch"}, 3, true},
		{"us inference", Usage{Model: "claude-opus-4-6", Input: M, Tier: "geo:us"}, 5.5, true},
		{"us inference before 4.6", Usage{Model: "claude-opus-4-5", Input: M, Tier: "geo:us"}, 0, false},
		{"priority", Usage{Model: "claude-opus-4-8", Input: M, Tier: "priority"}, 0, false},
		{"unknown region", Usage{Model: "claude-opus-4-8", Input: M, Tier: "geo:eu"}, 0, false},
		{"fable 5.1 cache reads", Usage{Model: "claude-fable-5-1", CacheRead: M}, 0.25, true},
		{"recorded zero cost", Usage{Model: "claude-opus-4-8", Input: M, CostKnown: true}, 0, true},
	} {
		got, ok := EstimateCost(tc.u)
		if ok != tc.ok || math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: %v %v, want %v %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

// A session that switched models is priced per response, never at the last
// model's rate; one unpriced response makes the total unknown.
func TestEventsCost(t *testing.T) {
	const M = 1_000_000
	opus := UsageEvent{Model: "claude-opus-4-8", Usage: Usage{Input: M}}
	haiku := UsageEvent{Model: "claude-haiku-4-5", Usage: Usage{Input: M}}
	if got, ok := EventsCost([]UsageEvent{opus, haiku}, AuthAPIKey); !ok || got != 6 {
		t.Errorf("opus then haiku = %v %v, want 6 (not 2 at haiku's rate)", got, ok)
	}
	// Claude Code logs API errors as "<synthetic>" responses with no tokens
	if got, ok := EventsCost([]UsageEvent{opus, {Model: "<synthetic>"}}, AuthAPIKey); !ok || got != 5 {
		t.Errorf("with a synthetic error line = %v %v, want 5", got, ok)
	}
	if _, ok := EventsCost([]UsageEvent{opus, {Model: "gpt-x", Usage: Usage{Input: M}}}, AuthAPIKey); ok {
		t.Error("an unpriced response must make the total unknown")
	}
	if _, ok := EventsCost([]UsageEvent{opus}, AuthSubscription); ok {
		t.Error("a subscription is not billed per token")
	}
}

func TestClaudeTier(t *testing.T) {
	for _, tc := range []struct{ speed, tier, geo, want string }{
		{"standard", "standard", "not_available", ""},
		{"", "", "global", ""},
		{"fast", "standard", "us", "fast,geo:us"},
		{"standard", "batch", "", "batch"},
		{"standard", "priority", "eu", "priority,geo:eu"},
	} {
		if got := claudeTier(tc.speed, tc.tier, tc.geo); got != tc.want {
			t.Errorf("claudeTier(%q,%q,%q) = %q, want %q", tc.speed, tc.tier, tc.geo, got, tc.want)
		}
	}
}
