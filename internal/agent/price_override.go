package agent

import (
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// PriceOverride is an exact fallback rate, never a wildcard or an exchange rate.
type PriceOverride struct {
	Agent        string   `yaml:"agent"`
	Provider     string   `yaml:"provider"`
	Model        string   `yaml:"model"`
	Tier         string   `yaml:"tier"`
	Billing      string   `yaml:"billing"`
	Currency     string   `yaml:"currency"`
	Source       string   `yaml:"source"`
	Date         string   `yaml:"date"`
	Input        *float64 `yaml:"input"`
	Output       *float64 `yaml:"output"`
	CacheRead    *float64 `yaml:"cacheRead"`
	CacheWrite   *float64 `yaml:"cacheWrite"`
	CacheWrite1h *float64 `yaml:"cacheWrite1h"`
}

func (p *PriceOverride) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("usage price entries must be mappings (values hidden)")
	}
	allowed := map[string]bool{}
	for _, key := range []string{"agent", "provider", "model", "tier", "billing", "currency", "source", "date", "input", "output", "cacheRead", "cacheWrite", "cacheWrite1h"} {
		allowed[key] = true
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !allowed[key] || seen[key] {
			return fmt.Errorf("usage price contains an unknown or duplicate field (values hidden)")
		}
		seen[key] = true
	}
	type plain PriceOverride
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return fmt.Errorf("usage price field types are invalid (values hidden)")
	}
	*p = PriceOverride(decoded)
	return nil
}

type CostSource struct {
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Date     string `json:"date,omitempty"`
	Currency string `json:"currency"`
	Agent    string `json:"agent,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Tier     string `json:"tier,omitempty"`
}

type CostEvidence struct {
	Amount float64
	Known  bool
	Origin CostSource
	Reason string
}

func ValidatePriceOverrides(rates []PriceOverride, now time.Time) error {
	if len(rates) > 256 {
		return fmt.Errorf("usage.prices accepts at most 256 exact rates")
	}
	seen := map[string]bool{}
	for i, r := range rates {

		bad := func(detail string) error { return fmt.Errorf("usage.prices entry %d: %s", i+1, detail) }
		safe := func(s string) bool {
			return len(s) <= 256 && !strings.ContainsFunc(s, func(c rune) bool { return unicode.IsControl(c) || unicode.IsSpace(c) })
		}
		if !slices.Contains([]string{"claude-code", "codex", "gemini-cli", "opencode", "pi"}, r.Agent) || r.Model == "" || !safe(r.Provider) || !safe(r.Model) || !safe(r.Tier) {
			return bad("use a supported usage agent and an exact model/provider/tier without whitespace or control characters")
		}
		if r.Billing != "api" || r.Currency != "USD" {
			return bad("billing must be api and currency must be USD; no currency conversion is performed")
		}
		if r.Input == nil || r.Output == nil {
			return bad("input and output rates are required, including explicit zero for free tokens")
		}
		u, err := url.Parse(r.Source)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || len(r.Source) > 2048 {
			return bad("source must be an HTTPS URL without credentials, query or fragment")
		}
		date, err := time.Parse("2006-01-02", r.Date)
		if err != nil || r.Date > now.UTC().Format("2006-01-02") || date.Year() < 2000 {
			return bad("date must be YYYY-MM-DD, from 2000 onward and not in the future")
		}
		for _, v := range []*float64{r.Input, r.Output, r.CacheRead, r.CacheWrite, r.CacheWrite1h} {
			if v != nil && (*v < 0 || math.IsNaN(*v) || math.IsInf(*v, 0) || *v > 1e9) {
				return bad("rates must be finite, nonnegative and at most one billion USD per million tokens")
			}
		}

		key := r.Agent + "\x00" + r.Provider + "\x00" + r.Model + "\x00" + r.Tier
		if seen[key] {
			return bad("agent/provider/model/tier must be unique; remove the duplicate fallback")
		}
		seen[key] = true
	}
	return nil
}

// ResolveCost retains recorded USD, then known API list prices, then exact user fallbacks.
// A fallback's billing: api is an explicit user declaration when historical auth is unknown.
func ResolveCost(e UsageEvent, mode AuthMode, rates []PriceOverride) CostEvidence {
	u := e.Usage
	if e.Model != "" {
		u.Model = e.Model
	}
	unavailable := func(reason string) CostEvidence { return CostEvidence{Reason: reason} }
	origin := CostSource{Currency: "USD", Agent: e.AgentID, Provider: e.Provider, Model: u.Model, Tier: u.Tier}
	if u.CostKnown || u.Cost > 0 {
		if u.Cost < 0 || math.IsNaN(u.Cost) || math.IsInf(u.Cost, 0) {
			return unavailable("invalid recorded cost")
		}
		origin.Kind, origin.Source = "recorded", "native transcript"
		return CostEvidence{Amount: u.Cost, Known: true, Origin: origin}
	}
	if u.Incomplete {
		return unavailable("token counters unavailable or unfinished")
	}
	if mode == AuthSubscription {
		return unavailable("subscription is not billed per token")
	}
	if mode == AuthAPIKey && u.Model == "<synthetic>" && u.Input == 0 && u.Output == 0 && u.CacheRead == 0 && u.CacheWrite == 0 {
		origin.Kind, origin.Source = "estimated", "zero token synthetic response"
		return CostEvidence{Known: true, Origin: origin}
	}
	if mode == AuthAPIKey {
		if cost, ok := EstimateCost(u); ok && u.Model != "" {
			// Zero counters do not give an unknown model a published price.
			model := strings.TrimSuffix(u.Model, "-latest")
			if i := strings.LastIndexByte(model, '-'); i >= 0 {
				if _, err := time.Parse("20060102", model[i+1:]); err == nil {
					model = model[:i]
				}
			}
			if _, exists := pricingTable[model]; exists {
				origin.Kind, origin.Source, origin.Date = "estimated", "https://platform.claude.com/docs/en/about-claude/pricing", "2026-09-30"
				return CostEvidence{Amount: cost, Known: true, Origin: origin}
			}
		}
	}
	for _, r := range rates {
		if r.Agent != e.AgentID || r.Provider != e.Provider || r.Model != u.Model || r.Tier != u.Tier || r.Billing != "api" {
			continue
		}
		cost := 0.0
		counts := []int{u.Input, u.Output, u.CacheRead, u.CacheWrite - min(u.CacheWrite1h, u.CacheWrite), min(u.CacheWrite1h, u.CacheWrite)}
		prices := []*float64{r.Input, r.Output, r.CacheRead, r.CacheWrite, r.CacheWrite1h}
		for i, n := range counts {
			if n < 0 || n > 0 && prices[i] == nil {
				return unavailable("a required token category has no configured rate")
			}
			if prices[i] != nil {
				cost += float64(n) * *prices[i] / 1e6
			}
		}
		if math.IsNaN(cost) || math.IsInf(cost, 0) {
			return unavailable("cost estimate is out of range")
		}
		origin.Kind, origin.Source, origin.Date = "estimated", r.Source, r.Date
		return CostEvidence{Amount: cost, Known: true, Origin: origin}
	}
	return unavailable("no applicable API price or historical billing declaration")
}
