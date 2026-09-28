package agent

import (
	"strings"
	"time"
)

// pricePerMTok is the USD price per million tokens.
type pricePerMTok struct {
	Input, Output, CacheRead, CacheWrite float64
}

// Claude API list prices, checked 2026-09-22:
// https://platform.claude.com/docs/en/about-claude/pricing
// Assumes 5-minute cache writes; ignores fast, batch and regional pricing.
// Explicit ids keep one version's price from applying to another.
var pricingTable = map[string]pricePerMTok{
	"claude-opus-4-1":   {Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75},
	"claude-opus-4-5":   {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"claude-opus-4-6":   {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"claude-opus-4-7":   {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"claude-opus-4-8":   {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"claude-opus-5":     {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"claude-opus-5-5":   {Input: 4, Output: 20, CacheRead: 0.2, CacheWrite: 5},
	"claude-sonnet-4-5": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-sonnet-4-6": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-sonnet-5":   {Input: 2, Output: 10, CacheRead: 0.2, CacheWrite: 2.5},
	"claude-haiku-4-5":  {Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25},
	"claude-opus-4":     {Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75},
	"claude-3-opus":     {Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75},
	"claude-sonnet-4":   {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-3-5-sonnet": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-3-7-sonnet": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-haiku-4":    {Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25},
	"claude-3-5-haiku":  {Input: 0.8, Output: 4, CacheRead: 0.08, CacheWrite: 1},
}

// CostFor is the cost shown to the user: only API-key accounts pay per token,
// so any other auth mode has no cost (ok=false), like an unknown model.
func CostFor(u Usage, mode AuthMode) (float64, bool) {
	if mode != AuthAPIKey {
		return 0, false
	}
	return EstimateCost(u)
}

// EstimateCost returns the USD cost of u when the model (optionally with a date
// suffix) is in the table. ok=false: unknown model; callers show tokens only.
func EstimateCost(u Usage) (cost float64, ok bool) {
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
	return float64(u.Input)/1e6*p.Input + float64(u.Output)/1e6*p.Output +
		float64(u.CacheRead)/1e6*p.CacheRead + float64(u.CacheWrite)/1e6*p.CacheWrite, true
}
