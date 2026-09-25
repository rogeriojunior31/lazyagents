package usage

import (
	"slices"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// Usage tab filters: period, agent, view and text. All in memory over the
// loaded events: changing a filter never rereads transcripts.

// period is a period filter option; since follows the CLI's --since.
type period struct{ id, label, since string }

var periods = []period{
	{"today", "today", "1d"},
	{"7d", "7 days", "7d"},
	{"30d", "30 days", "30d"},
	{"90d", "90 days", "90d"},
	{"all", "all", ""},
}

// tabViews are the table views in ←/→ order; ids are the CLI's.
var tabViews = []struct{ id, label string }{
	{"daily", "day"},
	{"agents", "agent"},
	{"projects", "project"},
	{"models", "model"},
}

// config is the `usage:` section of config.yaml: the filters the tab opens with.
type config struct {
	Period string `yaml:"period"` // today | 7d | 30d | 90d | all
	View   string `yaml:"view"`   // daily | agents | projects | models
}

// filters is the tab's filter state.
type filters struct {
	period int    // index into periods
	view   int    // index into tabViews
	agent  string // "" = all
	text   string // row filter; "" = none
}

// newFilters applies the config; unknown values keep the default (7 days, day).
func newFilters(cfg config) filters {
	f := filters{period: 1}
	if i := slices.IndexFunc(periods, func(p period) bool { return p.id == cfg.Period }); i >= 0 {
		f.period = i
	}
	if i := slices.IndexFunc(tabViews, func(v struct{ id, label string }) bool { return v.id == cfg.View }); i >= 0 {
		f.view = i
	}
	return f
}

// from is the period start; "all" starts at the first event.
func (f filters) from(events []agent.UsageEvent, now time.Time) time.Time {
	if p := periods[f.period]; p.since != "" {
		t, _, _ := parseSince(p.since, now)
		return t
	}
	if len(events) == 0 {
		return now
	}
	first := events[0].Time // events arrive in chronological order
	y, m, d := first.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// apply returns the events of the chosen period and agent. Events are
// chronological: the period is a binary search and, without an agent filter,
// just a slice (history can hold hundreds of thousands).
func (f filters) apply(events []agent.UsageEvent, now time.Time) []agent.UsageEvent {
	from := f.from(events, now)
	i, _ := slices.BinarySearchFunc(events, from, func(e agent.UsageEvent, t time.Time) int { return e.Time.Compare(t) })
	events = events[i:]
	if f.agent == "" {
		return events
	}
	var out []agent.UsageEvent
	for _, e := range events {
		if e.AgentID == f.agent {
			out = append(out, e)
		}
	}
	return out
}

// rows builds the view's rows; the text filter matches the label,
// case-insensitively. Days go newest first.
func (f filters) rows(events []agent.UsageEvent, price Pricer, from, now time.Time) []Total {
	var rows []Total
	switch tabViews[f.view].id {
	case "daily":
		rows = fillDays(Daily(events, 0, price), from, now)
		slices.Reverse(rows)
	case "agents":
		rows = ByAgent(events, price)
	case "projects":
		rows = ByProject(events, price)
	case "models":
		rows = ByModel(events, price)
	}
	if f.text == "" {
		return rows
	}
	q := strings.ToLower(f.text)
	return slices.DeleteFunc(rows, func(t Total) bool {
		label := t.Label
		if tabViews[f.view].id == "daily" {
			label = dayText(t.Label) // "Tue 09-23" matches too
		}
		return !strings.Contains(strings.ToLower(label), q)
	})
}

// agentsIn lists agents with events or limits, in limit order (registration)
// and then event order.
func agentsIn(sts []Status, events []agent.UsageEvent) []string {
	var ids []string
	for _, st := range sts {
		if !slices.Contains(ids, st.AgentID) {
			ids = append(ids, st.AgentID)
		}
	}
	for _, e := range events {
		if !slices.Contains(ids, e.AgentID) {
			ids = append(ids, e.AgentID)
		}
	}
	return ids
}

// cycle returns the next (step 1) or previous (-1) of "" + ids.
func cycle(cur string, ids []string, step int) string {
	opts := append([]string{""}, ids...)
	i := max(0, slices.Index(opts, cur))
	return opts[(i+step+len(opts))%len(opts)]
}
