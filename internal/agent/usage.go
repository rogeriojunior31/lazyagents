package agent

import (
	"encoding/json"
	"os"
	"time"
)

// Usage is the aggregated token usage of a session.
type Usage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	// CacheWrite1h is the part of CacheWrite written to the 1-hour cache,
	// priced 2x input instead of 1.25x.
	CacheWrite1h int
	// Tier lists the non-standard pricing the responses ran under, comma
	// separated: "fast", "batch", "priority", "geo:us"… ("" = standard).
	Tier      string
	Cost      float64 // USD the agent recorded itself (Pi); see CostKnown
	CostKnown bool    // Cost is the agent's own figure, even 0 (a local model, a subscription); false = estimate from the price table
	Model     string  // last model seen (best-effort)
}

// UsageReader is implemented by adapters that can sum a session's token usage.
// Optional (not every agent records usage), by type assertion.
type UsageReader interface {
	// SessionUsage sums the session usage. ok=false when the agent or the
	// transcript does not record it.
	SessionUsage(s Session) (Usage, bool)
}

// UsageEvent is one model response's token usage at a point in time, the input
// of the usage module's windows.
type UsageEvent struct {
	AgentID string // set by the aggregator
	Time    time.Time
	Model   string
	CWD     string
	Usage   Usage
	N       int // responses summed in this event (the index buckets them); 0 counts as 1
}

// UsageEventReader is implemented by adapters that record timestamped usage in
// the transcript. Optional, by type assertion.
type UsageEventReader interface {
	// UsageEvents returns the session's usage events in file order; none is an
	// empty list, not an error.
	UsageEvents(s Session) ([]UsageEvent, error)
}

// AuthMode is how the agent is authenticated; it decides whether a USD cost
// estimate makes sense (subscriptions are not billed per token).
type AuthMode int

const (
	AuthUnknown AuthMode = iota
	AuthSubscription
	AuthAPIKey
)

func (m AuthMode) String() string {
	switch m {
	case AuthSubscription:
		return "subscription"
	case AuthAPIKey:
		return "API key"
	default:
		return "unknown"
	}
}

// AuthModeReader is implemented by adapters that know how they are
// authenticated. detail is short and never secret (e.g. the plan).
type AuthModeReader interface {
	AuthMode() (mode AuthMode, detail string)
}

// secret records that a secret field exists without keeping its value: this
// package's decoders never hold tokens, keys or passwords in memory.
type secret bool

func (s *secret) UnmarshalJSON(b []byte) error {
	*s = secret(string(b) != "null" && string(b) != `""`)
	return nil
}

// decodeJSONFile streams path into v without holding the whole file.
func decodeJSONFile(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}
