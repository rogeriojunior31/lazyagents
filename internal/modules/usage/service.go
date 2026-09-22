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

// fresh diz se a entrada de cache ainda vale.
func fresh(c cacheEntry, now time.Time, ttl time.Duration) bool {
	return !c.FetchedAt.IsZero() && now.Sub(c.FetchedAt) < ttl
}

func (s *Service) readCache() map[string]cacheEntry {
	out := map[string]cacheEntry{}
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		return out
	}
	if json.Unmarshal(data, &out) != nil {
		return map[string]cacheEntry{} // cache corrompido é descartado
	}
	return out
}

func (s *Service) writeCache(c map[string]cacheEntry) {
	data, err := json.MarshalIndent(c, "", "  ")
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
