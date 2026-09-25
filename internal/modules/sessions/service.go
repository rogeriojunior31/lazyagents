// Package sessions unifica as sessões de todos os agentes instalados numa
// lista única ordenada por data, e resolve o comando de resume de cada uma.
//
// O módulo inteiro vive aqui: service (service.go, alias.go, export.go,
// search.go), aba da TUI (tab.go e os arquivos por assunto; os que repetem o
// assunto de um arquivo de domínio levam o sufixo _ui), CLI (cli.go) e o
// registro (feature.go).
package sessions

import (
	"errors"
	"fmt"
	"sort"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Service agrega os adapters, exporta transcripts e persiste apelidos.
// Exclusão explícita de sessões é delegada ao adapter, com backup.
type Service struct {
	adapters    []agent.Adapter
	backupsDir  string
	exportsDir  string
	aliasesPath string
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, backupsDir: paths.BackupsDir(), exportsDir: paths.ExportsDir(), aliasesPath: paths.AliasesPath()}
}

// BackupsDir devolve o diretório onde as sessões deletadas são arquivadas.
func (s *Service) BackupsDir() string { return s.backupsDir }

// ExportsDir devolve o diretório onde os transcripts exportados são gravados.
func (s *Service) ExportsDir() string { return s.exportsDir }

// ExportTranscript exporta o transcript da sessão para Markdown em
// ExportsDir().
func (s *Service) ExportTranscript(sess agent.Session, entries []agent.Entry) (string, error) {
	return ExportMarkdown(sess, entries, s.exportsDir)
}

// List devolve as sessões de todos os agentes, mais recentes primeiro, com o
// apelido preenchido. Falha de um agente (ou do arquivo de apelidos) não
// derruba os demais — erros voltam agregados.
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
	if _, aliases, err := s.readAliases(); err != nil {
		errs = append(errs, err)
	} else {
		for i := range out {
			out[i].Alias = aliases[aliasKey(out[i])]
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	return out, errors.Join(errs...)
}

// Search varre os transcripts das sessões dadas em busca de query.
func (s *Service) Search(sessions []agent.Session, query string) ([]Match, error) {
	return SearchTranscripts(s.adapters, sessions, query)
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
		return nil, fmt.Errorf("unknown agent: %s", sess.AgentID)
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
		return fmt.Errorf("unknown agent: %s", sess.AgentID)
	}
	if s.IsLive(sess) {
		return fmt.Errorf("session in progress: close it before deleting")
	}
	return ad.DeleteSession(sess, s.backupsDir)
}
