// Package session unifica as sessões de todos os agentes instalados numa
// lista única ordenada por data, e resolve o comando de resume de cada uma.
package session

import (
	"errors"
	"fmt"
	"sort"

	"lazyskills/internal/agent"
)

// Service agrega os adapters. Read-only: nunca escreve nos dados dos CLIs.
type Service struct {
	adapters   []agent.Adapter
	backupsDir string
}

func New(adapters []agent.Adapter, backupsDir string) *Service {
	return &Service{adapters: adapters, backupsDir: backupsDir}
}

// BackupsDir devolve o diretório onde as sessões deletadas são arquivadas.
func (s *Service) BackupsDir() string { return s.backupsDir }

// List devolve as sessões de todos os agentes, mais recentes primeiro.
// Falha de um agente não derruba os demais — erros voltam agregados.
func (s *Service) List() ([]agent.Session, error) {
	var out []agent.Session
	var errs []error
	for _, ad := range s.adapters {
		sessions, err := ad.ListSessions()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ad.ID(), err))
		}
		out = append(out, sessions...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	return out, errors.Join(errs...)
}

// ResumeCmd delega ao adapter dono da sessão.
func (s *Service) ResumeCmd(sess agent.Session) (argv []string, dir string, ok bool) {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return nil, "", false
	}
	return ad.ResumeCmd(sess)
}

// Transcript delega ao adapter dono da sessão.
func (s *Service) Transcript(sess agent.Session) ([]agent.Entry, error) {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return nil, fmt.Errorf("agente desconhecido: %s", sess.AgentID)
	}
	return ad.Transcript(sess)
}

// SessionUsage delega ao adapter dono da sessão, se ele souber informar o uso
// de tokens (type assertion opcional — nem todo adapter implementa
// agent.UsageReader). ok=false = sem informação de uso disponível.
func (s *Service) SessionUsage(sess agent.Session) (agent.Usage, bool) {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return agent.Usage{}, false
	}
	ur, ok := ad.(agent.UsageReader)
	if !ok {
		return agent.Usage{}, false
	}
	return ur.SessionUsage(sess)
}

// IsLive delega ao adapter dono da sessão, se ele souber dizer (type
// assertion opcional — agent.LiveChecker). false = sem suporte ou não viva.
func (s *Service) IsLive(sess agent.Session) bool {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return false
	}
	lc, ok := ad.(agent.LiveChecker)
	return ok && lc.IsLive(sess)
}

// DeleteSession delega a deleção ao adapter dono da sessão. Recusa sessões em
// andamento (mesma fonte de verdade do badge "●" — IsLive) para não apagar o
// arquivo debaixo de um processo vivo.
func (s *Service) DeleteSession(sess agent.Session) error {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return fmt.Errorf("agente desconhecido: %s", sess.AgentID)
	}
	if s.IsLive(sess) {
		return fmt.Errorf("sessão em andamento — feche-a antes de deletar")
	}
	return ad.DeleteSession(sess, s.backupsDir)
}
