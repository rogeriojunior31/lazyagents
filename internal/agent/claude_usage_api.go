package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"time"
)

// Claude Code subscription limits, from the same endpoint as the CLI's /usage.
// Nothing is on disk, so it is an HTTP call: on demand only, never at boot, and
// cached by the caller.
const (
	claudeUsageURL  = "https://api.anthropic.com/api/oauth/usage?at_wall=1&skip_spend=1"
	claudeOAuthBeta = "oauth-2025-04-20"
	maxUsageBody    = 1 << 20
)

// claudeUsageResponse covers only what the tab shows.
type claudeUsageResponse struct {
	Limits []struct {
		Kind     string  `json:"kind"` // session | weekly_all | weekly_scoped
		Percent  float64 `json:"percent"`
		Severity string  `json:"severity"`
		ResetsAt string  `json:"resets_at"`
		Scope    *struct {
			Model *struct {
				DisplayName string `json:"display_name"`
			} `json:"model"`
		} `json:"scope"`
	} `json:"limits"`
	FiveHour *claudeLegacyWindow `json:"five_hour"`
	SevenDay *claudeLegacyWindow `json:"seven_day"`
}

// claudeLegacyWindow is the old shape, the fallback when limits[] is absent.
type claudeLegacyWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

// RateLimits fetches the subscription limit windows.
//
// This is the ONLY place lazyagents materializes the Claude Code token: read
// from .credentials.json, used in the Authorization header and dropped. Never
// shown, logged, persisted or kept in an exported struct.
func (c *Claude) RateLimits(ctx context.Context) (RateStatus, error) {
	var creds struct {
		OAuth struct {
			AccessToken      string `json:"accessToken"`
			ExpiresAt        int64  `json:"expiresAt"` // epoch milliseconds
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
	}
	if err := decodeJSONFile(filepath.Join(c.configDir(), ".credentials.json"), &creds); err != nil {
		return RateStatus{}, noLimitsYet("no Claude Code credentials: sign in with /login in the CLI")
	}
	if creds.OAuth.AccessToken == "" {
		return RateStatus{}, errors.New("Claude Code account has no OAuth session (API key accounts have no subscription limits)")
	}
	if creds.OAuth.ExpiresAt > 0 && time.UnixMilli(creds.OAuth.ExpiresAt).Before(time.Now()) {
		return RateStatus{}, errors.New("Claude Code session expired: open the CLI to renew it")
	}
	url := c.UsageURL
	if url == "" {
		url = claudeUsageURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return RateStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.OAuth.AccessToken)
	req.Header.Set("anthropic-beta", claudeOAuthBeta)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{
		Timeout: 15 * time.Second,
		// no redirects: the Authorization header never reaches another host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return RateStatus{}, fmt.Errorf("querying Claude Code usage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RateStatus{}, fmt.Errorf("querying Claude Code usage: HTTP %d", resp.StatusCode)
	}
	var body claudeUsageResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxUsageBody)).Decode(&body); err != nil {
		return RateStatus{}, fmt.Errorf("invalid Claude Code usage response: %w", err)
	}
	return RateStatus{
		Plan:      creds.OAuth.SubscriptionType,
		Windows:   claudeWindows(body),
		FetchedAt: time.Now(),
		Source:    "api",
	}, nil
}

// claudeWindows maps the response to RateWindows, preferring limits[].
func claudeWindows(body claudeUsageResponse) []RateWindow {
	var out []RateWindow
	for _, l := range body.Limits {
		w := RateWindow{UsedPercent: l.Percent, Severity: l.Severity, ResetsAt: parseTime(l.ResetsAt)}
		switch l.Kind {
		case "session":
			w.Kind, w.Label = WindowSession, "session 5h"
		case "weekly_all":
			w.Kind, w.Label = WindowWeekly, "week"
		case "weekly_scoped":
			w.Kind, w.Label = WindowWeeklyModel, "week"
			if l.Scope != nil && l.Scope.Model != nil && l.Scope.Model.DisplayName != "" {
				w.Label = "week · " + l.Scope.Model.DisplayName
			}
		default:
			continue
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		if w := body.FiveHour; w != nil {
			out = append(out, RateWindow{Kind: WindowSession, Label: "session 5h", UsedPercent: w.Utilization, ResetsAt: parseTime(w.ResetsAt)})
		}
		if w := body.SevenDay; w != nil {
			out = append(out, RateWindow{Kind: WindowWeekly, Label: "week", UsedPercent: w.Utilization, ResetsAt: parseTime(w.ResetsAt)})
		}
	}
	sortWindows(out)
	return out
}

// parseTime accepts the API ISO-8601; empty or invalid is zero.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// sortWindows orders session → week → week per model (display order).
func sortWindows(ws []RateWindow) {
	rank := map[string]int{WindowSession: 0, WindowWeekly: 1, WindowWeeklyModel: 2}
	sort.SliceStable(ws, func(i, j int) bool { return rank[ws[i].Kind] < rank[ws[j].Kind] })
}
