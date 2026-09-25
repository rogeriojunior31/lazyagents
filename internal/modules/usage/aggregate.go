package usage

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// BlockWindow is the session window used for blocks (Claude Code's 5h).
const BlockWindow = 5 * time.Hour

// Block is an activity window: it starts at the first event and lasts
// BlockWindow; a longer gap opens another block.
type Block struct {
	Start  time.Time   `json:"start"`
	End    time.Time   `json:"end"` // Start + BlockWindow
	Last   time.Time   `json:"last"`
	Usage  agent.Usage `json:"-"`
	Events int         `json:"events"`
	Active bool        `json:"active"` // the window still contains now
}

// Blocks groups (sorted) events into BlockWindow windows.
func Blocks(events []agent.UsageEvent, now time.Time) []Block {
	var out []Block
	for _, e := range events {
		if n := len(out); n > 0 && e.Time.Before(out[n-1].End) {
			b := &out[n-1]
			add(&b.Usage, e.Usage)
			b.Events += responses(e)
			b.Last = e.Time
			continue
		}
		out = append(out, Block{Start: e.Time, End: e.Time.Add(BlockWindow), Last: e.Time, Usage: e.Usage, Events: responses(e)})
	}
	for i := range out {
		out[i].Active = now.Before(out[i].End) && !now.Before(out[i].Start)
	}
	return out
}

// Current returns the block that contains now, if any.
func Current(blocks []Block) (Block, bool) {
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Active {
			return blocks[i], true
		}
	}
	return Block{}, false
}

// Total is a labeled aggregate (a day, project, agent or model).
type Total struct {
	Label  string      `json:"label"`
	Usage  agent.Usage `json:"-"`
	Events int         `json:"events"`
	Tokens int         `json:"tokens"`
	Cost   float64     `json:"-"` // sum of each event's cost
	Priced bool        `json:"-"` // Cost is valid: every event in the aggregate had a price
}

// Pricer estimates an event's cost; ok=false means no price (subscription
// account or model not in the table). nil means no cost estimate.
type Pricer func(agent.UsageEvent) (float64, bool)

// Daily sums per local day, newest first, at most n days.
func Daily(events []agent.UsageEvent, n int, price Pricer) []Total {
	// events are sorted: the day is only formatted when it changes
	var y, d int
	var mo time.Month
	var day string
	out := group(events, price, func(e agent.UsageEvent) (string, string) {
		if ey, em, ed := e.Time.Local().Date(); day == "" || ey != y || em != mo || ed != d {
			y, mo, d = ey, em, ed
			day = e.Time.Local().Format("2006-01-02")
		}
		return day, day
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label > out[j].Label })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// ByProject sums per CWD (labeled by basename), largest first.
func ByProject(events []agent.UsageEvent, price Pricer) []Total {
	return byTokens(group(events, price, func(e agent.UsageEvent) (string, string) {
		if e.CWD != "" {
			if base := filepath.Base(e.CWD); base != "." && base != "/" && base != "" {
				return filepath.Clean(e.CWD), base
			}
		}
		return "", "no project"
	}))
}

// ByAgent sums per agent, largest first.
func ByAgent(events []agent.UsageEvent, price Pricer) []Total {
	return byTokens(group(events, price, func(e agent.UsageEvent) (string, string) { return e.AgentID, e.AgentID }))
}

// ByModel sums per model, largest first.
func ByModel(events []agent.UsageEvent, price Pricer) []Total {
	return byTokens(group(events, price, func(e agent.UsageEvent) (string, string) {
		m := eventModel(e)
		if m == "" {
			m = "unknown"
		}
		return m, m
	}))
}

// Sum aggregates every event.
func Sum(events []agent.UsageEvent, price Pricer) Total {
	all := group(events, price, func(agent.UsageEvent) (string, string) { return "", "total" })
	if len(all) == 0 {
		return Total{Label: "total", Priced: price != nil}
	}
	return all[0]
}

// group sums events per key, in first-seen order. Cost is summed per event: an
// aggregate of different models never gets a single model's rate, and one
// unpriced event leaves the total unpriced.
func group(events []agent.UsageEvent, price Pricer, key func(agent.UsageEvent) (id, label string)) []Total {
	byKey := map[string]*Total{}
	var order []string
	for _, e := range events {
		id, label := key(e)
		t, ok := byKey[id]
		if !ok {
			t = &Total{Label: label, Priced: price != nil}
			byKey[id] = t
			order = append(order, id)
		}
		add(&t.Usage, e.Usage)
		t.Events += responses(e)
		if price != nil {
			c, ok := price(e)
			t.Cost += c
			t.Priced = t.Priced && ok
		}
	}
	out := make([]Total, 0, len(order))
	for _, k := range order {
		t := *byKey[k]
		t.Tokens = Tokens(t.Usage)
		out = append(out, t)
	}
	return out
}

// sumTotals sums already aggregated rows (the tab footer under a text filter).
func sumTotals(rows []Total, priced bool) Total {
	t := Total{Label: "total", Priced: priced}
	for _, r := range rows {
		add(&t.Usage, r.Usage)
		t.Events += r.Events
		t.Tokens += r.Tokens
		t.Cost += r.Cost
		t.Priced = t.Priced && r.Priced
	}
	return t
}

func byTokens(out []Total) []Total {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Label < out[j].Label // tie: stable order across runs
	})
	return out
}

// eventModel is the event's model, falling back to the usage's.
func eventModel(e agent.UsageEvent) string {
	if e.Model != "" {
		return e.Model
	}
	return e.Usage.Model
}

// responses is how many responses the event counts (the index buckets them).
func responses(e agent.UsageEvent) int { return max(1, e.N) }

// Tokens is a usage's total tokens (fresh input, output and cache).
func Tokens(u agent.Usage) int { return u.Input + u.Output + u.CacheRead + u.CacheWrite }

// Cost estimates the cost in USD. ok=false for subscription accounts (not
// billed per token) or models missing from the price table.
func Cost(u agent.Usage, mode agent.AuthMode) (float64, bool) {
	if mode != agent.AuthAPIKey {
		return 0, false
	}
	return agent.EstimateCost(u)
}

func add(dst *agent.Usage, src agent.Usage) {
	if Tokens(*dst) == 0 {
		dst.Model = src.Model
	} else if dst.Model != src.Model {
		dst.Model = "mixed" // one rate cannot represent an aggregate of different models
	}
	dst.Input += src.Input
	dst.Output += src.Output
	dst.CacheRead += src.CacheRead
	dst.CacheWrite += src.CacheWrite

}
