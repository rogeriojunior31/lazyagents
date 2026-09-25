// Package sessions merges the sessions of every installed agent into one list,
// newest first, and resolves how to resume each one.
package sessions

import (
	"errors"
	"fmt"
	"sort"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Service aggregates the adapters, exports transcripts and stores aliases.
// Deletion is delegated to the adapter, with a backup.
type Service struct {
	adapters    []agent.Adapter
	backupsDir  string
	exportsDir  string
	aliasesPath string
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, backupsDir: paths.BackupsDir(), exportsDir: paths.ExportsDir(), aliasesPath: paths.AliasesPath()}
}

// BackupsDir is where deleted sessions are archived.
func (s *Service) BackupsDir() string { return s.backupsDir }

// ExportsDir is where exported transcripts are written.
func (s *Service) ExportsDir() string { return s.exportsDir }

// ExportTranscript exports the session transcript to Markdown in ExportsDir.
func (s *Service) ExportTranscript(sess agent.Session, entries []agent.Entry) (string, error) {
	return ExportMarkdown(sess, entries, s.exportsDir)
}

// List returns every agent's sessions, newest first, with aliases filled in.
// A failing agent or alias file does not hide the others; errors are joined.
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

// Search looks for query in the given sessions' transcripts.
func (s *Service) Search(sessions []agent.Session, query string) ([]Match, error) {
	return SearchTranscripts(s.adapters, sessions, query)
}

// ResumeCmd delegates to the session's adapter.
func (s *Service) ResumeCmd(sess agent.Session) (argv []string, dir string, ok bool) {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return nil, "", false
	}
	return ad.ResumeCmd(sess)
}

// Transcript delegates to the session's adapter.
func (s *Service) Transcript(sess agent.Session) ([]agent.Entry, error) {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return nil, fmt.Errorf("unknown agent: %s", sess.AgentID)
	}
	return ad.Transcript(sess)
}

// SessionUsage delegates to the adapter when it implements agent.UsageReader;
// ok is false when no usage is known.
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

// IsLive delegates to the adapter when it implements agent.LiveChecker;
// false means unsupported or not live.
func (s *Service) IsLive(sess agent.Session) bool {
	ad := agent.ByID(s.adapters, sess.AgentID)
	if ad == nil {
		return false
	}
	lc, ok := ad.(agent.LiveChecker)
	return ok && lc.IsLive(sess)
}

// DeleteSession delegates to the adapter. Live sessions (IsLive, same source
// as the "●" badge) are refused so a running process keeps its file.
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
