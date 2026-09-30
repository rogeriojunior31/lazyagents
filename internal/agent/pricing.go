package agent

import (
	"strings"
	"time"
)

// pricePerMTok is the USD price per million tokens. Cache writes are derived
// from Input (5-minute 1.25x, 1-hour 2x), as the pricing page defines them.
type pricePerMTok struct {
	Input, Output, CacheRead float64
	Geo                      bool // Claude 4.6+: inference_geo "us" costs 1.1x
}

// Claude API list prices, checked 2026-09-30:
// https://platform.claude.com/docs/en/about-claude/pricing
// Explicit ids keep one version's price from applying to another.
var pricingTable = map[string]pricePerMTok{
	"claude-fable-5-1":  {Input: 10, Output: 50, CacheRead: 0.25, Geo: true},
	"claude-mythos-5-1": {Input: 10, Output: 50, CacheRead: 0.25, Geo: true},
	"claude-fable-5":    {Input: 10, Output: 50, CacheRead: 1, Geo: true},
	"claude-mythos-5":   {Input: 10, Output: 50, CacheRead: 1, Geo: true},
	"claude-opus-5-5":   {Input: 4, Output: 20, CacheRead: 0.2, Geo: true},
	"claude-opus-5":     {Input: 5, Output: 25, CacheRead: 0.5, Geo: true},
	"claude-opus-4-8":   {Input: 5, Output: 25, CacheRead: 0.5, Geo: true},
	"claude-opus-4-7":   {Input: 5, Output: 25, CacheRead: 0.5, Geo: true},
	"claude-opus-4-6":   {Input: 5, Output: 25, CacheRead: 0.5, Geo: true},
	"claude-opus-4-5":   {Input: 5, Output: 25, CacheRead: 0.5},
	"claude-opus-4-1":   {Input: 15, Output: 75, CacheRead: 1.5},
	"claude-opus-4":     {Input: 15, Output: 75, CacheRead: 1.5},
	"claude-sonnet-5-5": {Input: 2, Output: 10, CacheRead: 0.2, Geo: true},
	"claude-sonnet-5":   {Input: 2, Output: 10, CacheRead: 0.2, Geo: true},
	"claude-sonnet-4-6": {Input: 3, Output: 15, CacheRead: 0.3, Geo: true},
	"claude-sonnet-4-5": {Input: 3, Output: 15, CacheRead: 0.3},
	"claude-sonnet-4":   {Input: 3, Output: 15, CacheRead: 0.3},
	"claude-haiku-4-5":  {Input: 1, Output: 5, CacheRead: 0.1},
	"claude-3-5-haiku":  {Input: 0.8, Output: 4, CacheRead: 0.08},
	// retired, no longer on the page; list prices when they were sold
	"claude-3-opus":     {Input: 15, Output: 75, CacheRead: 1.5},
	"claude-3-5-sonnet": {Input: 3, Output: 15, CacheRead: 0.3},
	"claude-3-7-sonnet": {Input: 3, Output: 15, CacheRead: 0.3},
	"claude-haiku-4":    {Input: 1, Output: 5, CacheRead: 0.1},
}

// fastPricing is fast mode's input/output price; the cache multipliers apply
// on top. Other models have no fast mode, so a "fast" response there is unpriced.
var fastPricing = map[string][2]float64{
	"claude-opus-5-5": {8, 40},
	"claude-opus-5":   {10, 50},
	"claude-opus-4-8": {10, 50},
}

// CostFor is the cost shown to the user: only API-key accounts pay per token,
// so any other auth mode has no cost (ok=false), like an unknown model.
func CostFor(u Usage, mode AuthMode) (float64, bool) {
	if mode != AuthAPIKey {
		return 0, false
	}
	return EstimateCost(u)
}

// EstimateCost returns the USD cost of u: the one the agent recorded, else the
// list price of its model (optionally with a date suffix) and tier. ok=false
// when any of it has no published price (unknown model, priority tier, a
// region or mode the page does not price): callers show tokens only, never a
// guess.
func EstimateCost(u Usage) (cost float64, ok bool) {
	if u.CostKnown || u.Cost > 0 {
		return u.Cost, true
	}
	// No tokens cost nothing, whatever the model: Claude Code logs API errors
	// and interrupts as "<synthetic>" responses with zero usage.
	if u.Input == 0 && u.Output == 0 && u.CacheRead == 0 && u.CacheWrite == 0 {
		return 0, true
	}
	if u.Model == "" {
		return 0, false
	}
	model := strings.TrimSuffix(u.Model, "-latest")
	if i := strings.LastIndexByte(model, '-'); i >= 0 {
		if _, err := time.Parse("20060102", model[i+1:]); err == nil {
			model = model[:i]
		}
	}
	p, known := pricingTable[model]
	if !known {
		return 0, false
	}
	input, output, read := p.Input, p.Output, p.CacheRead
	mult := 1.0
	for _, t := range strings.Split(u.Tier, ",") {
		switch t {
		case "":
		case "fast":
			f, ok := fastPricing[model]
			if !ok {
				return 0, false
			}
			read *= f[0] / input // the cache multiplier applies to the fast input price
			input, output = f[0], f[1]
		case "batch":
			mult *= 0.5
		case "geo:us":
			if !p.Geo {
				return 0, false
			}
			mult *= 1.1
		default: // priority, other regions: not priced on the page
			return 0, false
		}
	}
	write1h := min(u.CacheWrite1h, u.CacheWrite)
	write5m := u.CacheWrite - write1h
	return mult * (float64(u.Input)*input + float64(u.Output)*output + float64(u.CacheRead)*read +
		float64(write5m)*1.25*input + float64(write1h)*2*input) / 1e6, true
}

// EventsCost prices each event at its own model and tier and adds them up:
// never the last model's rate over the total. One unpriced event makes the
// whole sum unknown instead of a partial figure.
func EventsCost(events []UsageEvent, mode AuthMode) (float64, bool) {
	if mode != AuthAPIKey || len(events) == 0 {
		return 0, false
	}
	total := 0.0
	for _, e := range events {
		u := e.Usage
		if u.Model == "" {
			u.Model = e.Model
		}
		c, ok := EstimateCost(u)
		if !ok {
			return 0, false
		}
		total += c
	}
	return total, true
}
