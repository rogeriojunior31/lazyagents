package agent

import (
	"context"
	"time"
)

// On subscription accounts the limit is a percentage of a window (session,
// week) with a reset time, not tokens or cost. Each adapter reads it from its
// own CLI's source.

// Known window kinds.
const (
	WindowSession     = "session"      // short window (5h in Claude Code)
	WindowWeekly      = "weekly"       // account-wide weekly window
	WindowWeeklyModel = "weekly_model" // weekly window of one model
)

type RateWindow struct {
	Kind        string    `json:"kind"`               // WindowSession | WindowWeekly | WindowWeeklyModel
	Label       string    `json:"label"`              // display text, may change between versions: match on Kind
	UsedPercent float64   `json:"used_percent"`       // 0–100
	ResetsAt    time.Time `json:"resets_at,omitzero"` // zero = unknown
	Severity    string    `json:"severity,omitempty"` // "normal", "warning"… best-effort
}

// RateStatus is an agent's limits at one moment.
type RateStatus struct {
	Plan      string       `json:"plan,omitempty"` // subscription plan, e.g. "max", "prolite"
	Windows   []RateWindow `json:"windows"`        // ordered: session, week, week per model
	FetchedAt time.Time    `json:"fetched_at,omitzero"`
	Source    string       `json:"source,omitempty"` // "api" (network) | "rollout" (local file)
}

// RateLimitReader is implemented by adapters that can report subscription
// limits. Optional. Network implementations honor ctx and never run at boot.
type RateLimitReader interface {
	RateLimits(ctx context.Context) (RateStatus, error)
}
