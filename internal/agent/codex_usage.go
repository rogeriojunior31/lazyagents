package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// The Codex rollout already carries usage and limits; nothing here uses the
// network. Per-response usage: `token_usage_record` (payload.usage), or in old
// versions `event_msg` with payload.type "token_count"
// (payload.info.last_token_usage); both can coexist in a file, so the legacy
// one is used only when there is no token_usage_record. Subscription limits:
// the last `token_count` with payload.rate_limits.

type codexUsageNumbers struct {
	InputTokens          int `json:"input_tokens"` // already includes cached_input_tokens
	CachedInputTokens    int `json:"cached_input_tokens"`
	CacheWriteInputToken int `json:"cache_write_input_tokens"`
	OutputTokens         int `json:"output_tokens"`
}

// usage converts to the app Usage, taking cache out of input so the cost
// estimate does not count the same tokens twice.
func (n codexUsageNumbers) usage(model string) Usage {
	return Usage{
		Input:      max(0, n.InputTokens-n.CachedInputTokens),
		Output:     n.OutputTokens,
		CacheRead:  n.CachedInputTokens,
		CacheWrite: n.CacheWriteInputToken,
		Model:      model,
	}
}

type codexUsageLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type  string             `json:"type"` // in event_msg: "token_count"
		CWD   string             `json:"cwd"`  // session_meta / turn_context
		Model string             `json:"model"`
		Usage *codexUsageNumbers `json:"usage"` // token_usage_record
		Info  *struct {
			LastTokenUsage *codexUsageNumbers `json:"last_token_usage"`
		} `json:"info"`
		RateLimits *codexRateLimits `json:"rate_limits"`
	} `json:"payload"`
}

type codexRateLimits struct {
	Primary   *codexWindow `json:"primary"`
	Secondary *codexWindow `json:"secondary"`
	PlanType  string       `json:"plan_type"`
}

type codexWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int     `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"` // epoch seconds
}

// UsageEvents returns one event per model response in the rollout. Model and
// cwd are not in the token event: they come from the last turn_context or
// session_meta before it.
func (c *Codex) UsageEvents(s Session) ([]UsageEvent, error) {
	e, err := c.index().refresh(s.Path, codexIndexLine)
	if err != nil {
		return nil, fmt.Errorf("reading rollout: %w", err)
	}
	if len(e.Events) == 0 {
		return e.events(e.Legacy, s), nil // old-version rollout
	}
	return e.events(e.Events, s), nil
}

// codexKeys are the substrings of a line the index needs: token events
// (token_usage_record, token_count, which also carries limits) or context
// (session_meta, turn_context: cwd and model). Messages, most of the rollout,
// are skipped without decoding.
var codexKeys = [][]byte{[]byte(`"token_`), []byte(`"cwd"`), []byte(`"model"`)}

// codexIndexLine extracts responses (with the model and cwd of the last
// context) and subscription limits from a rollout line.
func codexIndexLine(e *indexEntry, line []byte) {
	if !slices.ContainsFunc(codexKeys, func(k []byte) bool { return bytes.Contains(line, k) }) {
		return
	}
	var l codexUsageLine
	if json.Unmarshal(line, &l) != nil {
		return // unreadable line: best-effort
	}
	if l.Payload.Model != "" {
		e.Model = l.Payload.Model
	}
	if l.Payload.CWD != "" {
		e.CtxCWD = l.Payload.CWD
		if e.CWD == "" {
			e.CWD = l.Payload.CWD // session cwd (session_meta): buckets store only a different one
		}
	}
	ts, _ := time.Parse(time.RFC3339, l.Timestamp) // no timestamp: zero, the session time applies
	switch {
	case l.Type == "token_usage_record" && l.Payload.Usage != nil:
		e.addEvent(&e.Events, ts, e.Model, e.CtxCWD, l.Payload.Usage.usage(e.Model))
	case l.Payload.Type == "token_count" && l.Payload.Info != nil && l.Payload.Info.LastTokenUsage != nil:
		e.addEvent(&e.Legacy, ts, e.Model, e.CtxCWD, l.Payload.Info.LastTokenUsage.usage(e.Model))
	}
	if l.Payload.RateLimits != nil {
		rl := *l.Payload.RateLimits
		e.Rate, e.RateAt = &rl, 0
		if !ts.IsZero() {
			e.RateAt = ts.UnixNano()
		}
	}
}

// RateLimits reads the limits of the latest rollout that recorded them.
// Offline: Codex writes used_percent, the window and the reset every turn.
func (c *Codex) RateLimits(context.Context) (RateStatus, error) {
	sessions, err := c.ListSessions()
	if err != nil {
		return RateStatus{}, err
	}
	if len(sessions) == 0 {
		return RateStatus{}, noLimitsYet("no Codex session found")
	}
	// ListSessions returns newest first (and refreshes the index); a few files
	// are enough because every turn records the limits.
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		e, ok := c.index().get(s.Path)
		if !ok || e.Rate == nil {
			continue
		}
		var at time.Time
		if e.RateAt != 0 {
			at = time.Unix(0, e.RateAt)
		}
		return RateStatus{Plan: e.Rate.PlanType, Windows: codexWindows(*e.Rate), FetchedAt: at, Source: "rollout"}, nil
	}
	return RateStatus{}, errors.New("no limits recorded in recent Codex sessions")
}

// codexWindows maps Codex windows; the kind comes from window_minutes
// (10080 = week), not from the position, which varies per account.
func codexWindows(rl codexRateLimits) []RateWindow {
	var out []RateWindow
	for _, w := range []*codexWindow{rl.Primary, rl.Secondary} {
		if w == nil {
			continue
		}
		rw := RateWindow{Kind: WindowSession, Label: codexWindowLabel(w.WindowMinutes), UsedPercent: w.UsedPercent}
		if w.WindowMinutes >= 7*24*60 {
			rw.Kind = WindowWeekly
		}
		if w.ResetsAt > 0 {
			rw.ResetsAt = time.Unix(w.ResetsAt, 0)
		}
		out = append(out, rw)
	}
	sortWindows(out)
	return out
}

func codexWindowLabel(minutes int) string {
	switch {
	case minutes <= 0:
		return "window"
	case minutes >= 7*24*60:
		return fmt.Sprintf("week (%dd)", minutes/(24*60))
	case minutes >= 24*60:
		return fmt.Sprintf("%dd", minutes/(24*60))
	case minutes >= 60:
		return fmt.Sprintf("session %dh", minutes/60)
	default:
		return fmt.Sprintf("%dmin", minutes)
	}
}

// AuthMode reads only the mode in ~/.codex/auth.json; the key is never
// decoded, only its presence.
func (c *Codex) AuthMode() (AuthMode, string) {
	var auth struct {
		AuthMode string `json:"auth_mode"`
		APIKey   secret `json:"OPENAI_API_KEY"`
	}
	if err := decodeJSONFile(filepath.Join(c.configDir(), "auth.json"), &auth); err == nil {
		switch auth.AuthMode {
		case "chatgpt", "oauth":
			return AuthSubscription, "ChatGPT"
		case "apikey", "api_key":
			return AuthAPIKey, "auth.json"
		}
		if bool(auth.APIKey) {
			return AuthAPIKey, "auth.json"
		}
		if auth.AuthMode != "" {
			return AuthSubscription, auth.AuthMode
		}
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return AuthAPIKey, "OPENAI_API_KEY"
	}
	return AuthUnknown, ""
}
