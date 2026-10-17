// Package providers switches the endpoint/model each agent uses by applying
// named profiles to the CLI's live config (cc-switch style). Profiles live in
// <ConfigDir>/providers.json (0600, may hold tokens); writing each agent's file
// is the adapter's job, via agent.ProviderHost.
package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// maxNameLen keeps a profile name within the TUI matrix.
const maxNameLen = 40

// Service holds the profile library and applies profiles to agents.
type Service struct {
	adapters   []agent.Adapter
	path       string
	backupsDir string
	home       string // only to shorten paths on screen
	// Detect returns agent detection; callers pass the memoized version so
	// every CLI's --version does not run again. nil falls back to direct detection.
	Detect func() []agent.Agent
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, path: paths.ProvidersPath(), backupsDir: paths.BackupsDir(), home: paths.Home}
}

// detectAll runs detection once per operation, never per adapter: each
// Detect pays a --version call.
func (s *Service) detectAll() []agent.Agent {
	if s.Detect != nil {
		return s.Detect()
	}
	return agent.DetectAll(s.adapters)
}

func findAgent(agents []agent.Agent, id string) agent.Agent {
	for _, a := range agents {
		if a.ID == id {
			return a
		}
	}
	return agent.Agent{ID: id}
}

// Path is the profiles file (shown in doctor).
func (s *Service) Path() string { return s.path }

// library is the on-disk format: an object, not a list, so new fields fit
// without breaking existing files.
type library struct {
	Profiles []agent.ProviderProfile `json:"profiles"`
}

// Profiles returns the saved profiles in alphabetical order, token included:
// callers that display them use Redacted.
func (s *Service) Profiles() ([]agent.ProviderProfile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", s.path, err)
	}
	var lib library
	if err := json.Unmarshal(data, &lib); err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.path, err)
	}
	sort.Slice(lib.Profiles, func(i, j int) bool { return lib.Profiles[i].Name < lib.Profiles[j].Name })
	return lib.Profiles, nil
}

// Profile finds a profile by name.
func (s *Service) Profile(name string) (agent.ProviderProfile, error) {
	profiles, err := s.Profiles()
	if err != nil {
		return agent.ProviderProfile{}, err
	}
	for _, p := range profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return agent.ProviderProfile{}, fmt.Errorf("profile %q does not exist", name)
}

// Save creates or replaces a profile by name.
func (s *Service) Save(p agent.ProviderProfile) error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return fmt.Errorf("the profile needs a name")
	case len([]rune(p.Name)) > maxNameLen:
		return fmt.Errorf("profile name: at most %d characters", maxNameLen)
	case p.BaseURL == "" && p.Model == "" && p.Token == "":
		return fmt.Errorf("profile %q changes nothing: set baseUrl, model or token", p.Name)
	}
	if err := agent.ValidateProviderProfile(p); err != nil {
		return err
	}
	p.HasToken = false // derived; never persisted

	profiles, err := s.Profiles()
	if err != nil {
		return err
	}
	replaced := false
	for i := range profiles {
		if profiles[i].Name == p.Name {
			profiles[i], replaced = p, true
			break
		}
	}
	if !replaced {
		profiles = append(profiles, p)
	}
	return s.write(profiles)
}

// Edit rewrites profile orig with p's fields. An empty token keeps the saved
// one: the tab never sees the token, so it cannot send it back. A different
// name renames the profile.
func (s *Service) Edit(orig string, p agent.ProviderProfile) error {
	old, err := s.Profile(orig)
	if err != nil {
		return err
	}
	if p.Token == "" {
		p.Token = old.Token
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name != orig {
		if _, err := s.Profile(p.Name); err == nil {
			return fmt.Errorf("profile %q already exists", p.Name)
		}
	}
	if err := s.Save(p); err != nil {
		return err
	}
	if p.Name != orig {
		return s.Delete(orig)
	}
	return nil
}

// Delete removes a profile from the library. Agents where it was applied are
// left alone; that is what Clear is for.
func (s *Service) Delete(name string) error {
	profiles, err := s.Profiles()
	if err != nil {
		return err
	}
	kept := profiles[:0]
	for _, p := range profiles {
		if p.Name != name {
			kept = append(kept, p)
		}
	}
	if len(kept) == len(profiles) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	return s.write(kept)
}

func (s *Service) write(profiles []agent.ProviderProfile) error {
	data, err := json.MarshalIndent(library{Profiles: profiles}, "", "  ")
	if err != nil {
		return fmt.Errorf("writing profiles: %w", err)
	}
	// 0600: the file may hold tokens (rule 7).
	return fsutil.WriteAtomic(s.path, append(data, '\n'), 0o600)
}

// Status is what is applied in an agent right now.
type Status struct {
	AgentID   string   `json:"agent"`
	AgentName string   `json:"name"`
	Short     string   `json:"-"` // letter of the agent column in the TUI
	File      string   `json:"file"`
	Files     []string `json:"-"`
	Installed bool     `json:"installed"`
	// Applied is the provider read from the live config, never with the token.
	Applied agent.ProviderProfile `json:"applied,omitempty"`
	Active  bool                  `json:"active"`
	// Profile is the library profile matching Applied (empty when configured
	// outside lazyagents).
	Profile string `json:"profile,omitempty"`
	Err     string `json:"error,omitempty"`
}

// Status returns one Status per agent that supports provider switching, in
// registration order.
func (s *Service) Status() []Status {
	profiles, _ := s.Profiles()
	agents := s.detectAll()
	var out []Status
	for _, ad := range s.adapters {
		host, ok := ad.(agent.ProviderHost)
		if !ok {
			continue
		}
		a := findAgent(agents, ad.ID())
		st := Status{AgentID: ad.ID(), AgentName: a.Name, Short: a.Short, File: host.ProviderFile(), Files: agent.ProviderFiles(host), Installed: a.Installed}
		applied, active, err := host.ReadProvider()
		switch {
		case err != nil:
			st.Err = err.Error()
		case active:
			st.Applied, st.Active = applied.Redacted(), true
			st.Profile = matchProfile(applied, profiles)
		}
		out = append(out, st)
	}
	return out
}

// matchProfile identifies the applied profile by endpoint (which every agent
// writes); the model breaks ties between profiles sharing an endpoint.
func matchProfile(applied agent.ProviderProfile, profiles []agent.ProviderProfile) string {
	best := ""
	for _, p := range profiles {
		if p.BaseURL == "" || p.BaseURL != applied.BaseURL {
			continue
		}
		if p.Model == applied.Model {
			return p.Name
		}
		if best == "" {
			best = p.Name
		}
	}
	return best
}

// Apply writes the profile to the agent. An empty agentID applies it to every
// installed agent that supports it.
func (s *Service) Apply(name, agentID string) error {
	return s.ApplyResult(name, core.TargetIDs(agentID)).AsError()
}

// ApplyResult accepts nil for all installed targets, or an explicit selection.
func (s *Service) ApplyResult(name string, ids []string) core.OperationResult {
	p, err := s.Profile(name)
	if err != nil {
		return core.OperationResult{Err: err}
	}
	_, validation := s.prepare(p, ids)
	if validation.AsError() != nil {
		if validation.Err == nil {
			validation.Err = fmt.Errorf("provider validation failed before writing; no agent files were changed")
		}
		// All selected targets remain retryable: none were written during preflight.
		for i := range validation.Targets {
			t := &validation.Targets[i]
			if t.Outcome == core.Succeeded {
				t.Outcome, t.Detail = core.Failed, "not attempted: another selected agent failed validation; fix the selection before applying"
			}
		}
		return validation
	}
	// Pin the resolved selection: installation/detection cannot expand it after review.
	selected := make([]string, 0, len(validation.Targets))
	for _, t := range validation.Targets {
		if t.Outcome == core.Succeeded {
			selected = append(selected, t.ID)
		}
	}
	applied := s.each(selected, func(_ string, host agent.ProviderHost) error { return host.ApplyProvider(p, s.backupsDir) })
	for i := range validation.Targets {
		for _, t := range applied.Targets {
			if t.ID == validation.Targets[i].ID {
				validation.Targets[i] = t
			}
		}
	}
	return validation
}

// Clear undoes what lazyagents applied, keeping the rest of the file.
func (s *Service) Clear(agentID string) error {
	return s.ClearResult(core.TargetIDs(agentID)).AsError()
}

func (s *Service) ClearResult(ids []string) core.OperationResult {
	return s.each(ids, func(_ string, host agent.ProviderHost) error { return host.ClearProvider(s.backupsDir) })
}

func (s *Service) each(ids []string, fn func(string, agent.ProviderHost) error) core.OperationResult {
	var result core.OperationResult
	var agents []agent.Agent
	if ids == nil {
		agents = s.detectAll()
	}
	attempted := false
	seen := map[string]bool{}
	for _, ad := range s.adapters {
		id := ad.ID()
		if ids != nil && !slices.Contains(ids, id) {
			continue
		}
		seen[id] = true
		t := core.TargetResult{ID: id, Outcome: core.Skipped}
		host, ok := ad.(agent.ProviderHost)
		switch {
		case !ok:
			t.Detail = "does not support switching providers"
		case ids == nil && !findAgent(agents, id).Installed:
			t.Detail = "not installed"
		default:
			attempted = true
			t.Outcome = core.Succeeded
			if err := fn(id, host); err != nil {
				t.Outcome, t.Detail = core.Failed, err.Error()
			}
			applied, active, err := host.ReadProvider()
			switch {
			case err != nil:
				t.Outcome = core.Failed
				t.Detail = strings.TrimSpace(t.Detail + " verification: " + err.Error())
				t.State = "unreadable"
			case !active:
				t.State = "agent default"
			default:
				// Report presence only: no endpoint, token or raw config enters a result.
				t.State = "provider configured"
				if profiles, err := s.Profiles(); err == nil {
					if name := matchProfile(applied, profiles); name != "" {
						t.State = fmt.Sprintf("profile %q configured", name)
					}
				}
			}
		}
		if ids != nil && t.Outcome == core.Skipped {
			t.Outcome = core.Failed
		}
		result.Targets = append(result.Targets, t)
	}
	for _, id := range ids {
		if !seen[id] {
			result.Targets = append(result.Targets, core.TargetResult{ID: id, Outcome: core.Failed, Detail: fmt.Sprintf("agent %q does not exist", id)})
		}
	}
	if !attempted && ids == nil {
		result.Err = fmt.Errorf("no installed agent supports switching providers")
	}
	return result
}
