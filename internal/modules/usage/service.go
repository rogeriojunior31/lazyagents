// Package usage responde "quanto já usei e quanto falta" para cada agente.
//
// Em conta por assinatura o que importa são as janelas de limite (sessão e
// semana, em percentual, com horário de reset), lidas por
// agent.RateLimitReader. Tokens e custo em USD são o detalhe, e o custo só
// faz sentido em conta por chave de API.
//
// Nada aqui roda no boot: Status faz rede (no adapter do Claude Code) e por
// isso é sob demanda e cacheado em disco.
//
// O módulo inteiro vive aqui: service (service.go, aggregate.go), aba
// (tab.go, view.go, help.go), CLI (cli.go) e registro (feature.go).
package usage

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// DefaultTTL é a validade do cache de limites.
const DefaultTTL = 5 * time.Minute

// Service agrega os adapters que sabem informar uso e limites.
type Service struct {
	adapters  []agent.Adapter
	cachePath string
	TTL       time.Duration
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, cachePath: paths.UsageCachePath(), TTL: DefaultTTL}
}

// Status é a situação de um agente: como está autenticado e como estão as
// janelas de limite. Err preenchido = falhou só este agente.
type Status struct {
	AgentID    string           `json:"agent"`
	Auth       agent.AuthMode   `json:"-"`
	AuthLabel  string           `json:"auth"`
	AuthDetail string           `json:"auth_detail,omitempty"`
	Limits     agent.RateStatus `json:"limits"`
	Cached     bool             `json:"cached"`
	Err        string           `json:"error,omitempty"`
}

// Status devolve a situação de todos os agentes que sabem informá-la, em
// ordem de registro. refresh=true ignora o cache.
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
			continue // agente sem noção de limite não aparece na aba
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
			st.Err = err.Error()
			if c, ok := cache[ad.ID()]; ok { // stale é melhor que nada
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

// cacheEntry é o que vai para o disco: o próprio status de limites.
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

// fresh diz se a entrada de cache ainda vale.
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
	_ = fsutil.WriteAtomic(s.cachePath, data, 0o644) // cache: falha não é erro de usuário
}

// Events junta os eventos de uso das sessões dadas, em ordem cronológica.
// Sessão de agente sem UsageEventReader é ignorada.
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
			continue // best-effort: sessão ilegível não derruba o resto
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

// RecentEvents junta os eventos de uso a partir de since, lendo só as sessões
// modificadas desde então. agentID vazio = todos os agentes.
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
			continue // best-effort, como Events
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

// apiKeyAgents são os agentes autenticados por API key: os únicos em que
// estimar custo por token faz sentido (assinatura não é cobrada por token).
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

// pricerFor estima o custo de cada evento dos agentes em api. nil = nenhum
// agente cobra por token.
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
