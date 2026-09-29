// Package usage answers "how much have I used and how much is left" per agent.
//
// For subscription accounts what matters is the limit windows (session and
// week, in percent, with reset time), read through agent.RateLimitReader.
// Tokens and USD cost are detail; cost only makes sense for API key accounts.
//
// Nothing here runs at boot: Status hits the network (Claude Code adapter),
// so it is on demand and cached on disk.
package usage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// DefaultTTL is how long cached limits stay valid.
const DefaultTTL = 5 * time.Minute

// Service aggregates the adapters that can report usage and limits.
type Service struct {
	adapters  []agent.Adapter
	cachePath string
	TTL       time.Duration
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, cachePath: paths.UsageCachePath(), TTL: DefaultTTL}
}

// Status is an agent's situation: how it is authenticated and its limit
// windows. A non-empty Err means only this agent failed.
type Status struct {
	AgentID    string           `json:"agent"`
	Auth       agent.AuthMode   `json:"-"`
	AuthLabel  string           `json:"auth"`
	AuthDetail string           `json:"auth_detail,omitempty"`
	Limits     agent.RateStatus `json:"limits"`
	Cached     bool             `json:"cached"`
	Err        string           `json:"error,omitempty"`
	// NoData: Err is agent.ErrNoLimitsYet (not signed in, never used), not a
	// failure; such an agent is listed only when a stale cache exists.
	NoData bool `json:"no_data,omitempty"`
}

// Status returns the situation of every agent that can report it, in
// registration order. refresh=true skips the cache.
func (s *Service) Status(ctx context.Context, refresh bool) []Status {
	cache := s.readCache()
	var out []Status
	now := time.Now()
	for _, ad := range s.adapters {
		st := Status{AgentID: ad.ID(), AuthLabel: agent.AuthUnknown.String()}
		if am, ok := ad.(agent.AuthModeReader); ok {
			st.Auth, st.AuthDetail = am.AuthMode()
			st.AuthLabel = st.Auth.String()
		}
		rl, ok := ad.(agent.RateLimitReader)
		if !ok {
			continue // an agent without limits does not show up in the tab
		}
		if !refresh {
			if c, ok := cache[ad.ID()]; ok && fresh(c, now, s.ttl()) {
				st.Limits, st.Cached = c, true
				out = append(out, st)
				continue
			}
		}
		limits, err := rl.RateLimits(ctx)
		if err != nil {
			st.Err, st.NoData = err.Error(), errors.Is(err, agent.ErrNoLimitsYet)
			c, cached := cache[ad.ID()]
			if st.NoData && !cached {
				continue // not installed, not signed in or never used: nothing to show
			}
			if cached { // stale beats nothing
				st.Limits, st.Cached = c, true
			}
			out = append(out, st)
			continue
		}
		st.Limits = limits
		cache[ad.ID()] = limits
		out = append(out, st)
	}
	s.writeCache(cache)
	return out
}

func (s *Service) ttl() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return DefaultTTL
}

// cacheEntry is what goes to disk: the limit status itself.
type cacheEntry = agent.RateStatus

// cacheVersion changes when a cached status would be shown differently (v2:
// window labels in English). An older cache, even stale, is never shown.
const cacheVersion = 2

// cacheFile is the on-disk shape. Before v2 the file was the bare agent map,
// which decodes here as version 0.
type cacheFile struct {
	Version int                   `json:"version"`
	Agents  map[string]cacheEntry `json:"agents"`
}

// fresh reports whether a cache entry is still valid.
func fresh(c cacheEntry, now time.Time, ttl time.Duration) bool {
	return !c.FetchedAt.IsZero() && !c.FetchedAt.After(now) && now.Sub(c.FetchedAt) < ttl
}

func (s *Service) readCache() map[string]cacheEntry {
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		return map[string]cacheEntry{}
	}
	var f cacheFile
	if json.Unmarshal(data, &f) != nil || f.Version != cacheVersion || f.Agents == nil {
		return map[string]cacheEntry{} // corrupt or outdated cache is dropped
	}
	return f.Agents
}

func (s *Service) writeCache(c map[string]cacheEntry) {
	data, err := json.MarshalIndent(cacheFile{Version: cacheVersion, Agents: c}, "", "  ")
	if err != nil {
		return
	}
	_ = fsutil.WriteAtomic(s.cachePath, data, 0o644) // cache: failing is not a user error
}

// Events gathers usage events from the given sessions, in chronological
// order. Sessions of agents without UsageEventReader are skipped.
func (s *Service) Events(sessions []agent.Session) []agent.UsageEvent {
	var out []agent.UsageEvent
	for _, sess := range sessions {
		ad := agent.ByID(s.adapters, sess.AgentID)
		ur, ok := ad.(agent.UsageEventReader)
		if !ok {
			continue
		}
		evs, err := ur.UsageEvents(sess)
		if err != nil {
			continue // best-effort: an unreadable session does not sink the rest
		}
		for _, e := range evs {
			if e.AgentID == "" {
				e.AgentID = sess.AgentID
			}
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// RecentEvents gathers usage events from since, reading only sessions
// modified since then. An empty agentID means every agent.
func (s *Service) RecentEvents(since time.Time, agentID string) []agent.UsageEvent {
	var sessions []agent.Session
	for _, ad := range s.adapters {
		if agentID != "" && ad.ID() != agentID {
			continue
		}
		if _, ok := ad.(agent.UsageEventReader); !ok {
			continue
		}
		list, err := ad.ListSessions()
		if err != nil {
			continue // best-effort, like Events
		}
		for _, sess := range list {
			if !sess.MTime.Before(since) {
				sessions = append(sessions, sess)
			}
		}
	}
	var out []agent.UsageEvent
	for _, e := range s.Events(sessions) {
		if !e.Time.Before(since) {
			out = append(out, e)
		}
	}
	return out
}

// apiKeyAgents are the agents authenticated by API key: the only ones where
// estimating per-token cost makes sense (subscriptions are not billed per token).
func (s *Service) apiKeyAgents() map[string]bool {
	api := map[string]bool{}
	for _, ad := range s.adapters {
		if am, ok := ad.(agent.AuthModeReader); ok {
			if mode, _ := am.AuthMode(); mode == agent.AuthAPIKey {
				api[ad.ID()] = true
			}
		}
	}
	return api
}

// pricerFor prices each event of the agents in api. nil means no agent
// is billed per token.
func pricerFor(api map[string]bool) Pricer {
	if len(api) == 0 {
		return nil
	}
	return func(e agent.UsageEvent) (float64, bool) {
		if !api[e.AgentID] {
			return 0, false
		}
		u := e.Usage
		u.Model = eventModel(e)
		return agent.EstimateCost(u)
	}
}
