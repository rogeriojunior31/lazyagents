// Package providers switches the endpoint/model each agent uses by applying
// named profiles to the CLI's live config (cc-switch style). Profiles live in
// <ConfigDir>/providers.json (0600, may hold tokens); writing each agent's file
// is the adapter's job, via agent.ProviderHost.
package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
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
	case p.BaseURL != "" && !validURL(p.BaseURL):
		return fmt.Errorf("endpoint %q: use an http:// or https:// URL", p.BaseURL)
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
	AgentID   string `json:"agent"`
	AgentName string `json:"name"`
	Short     string `json:"-"` // letter of the agent column in the TUI
	File      string `json:"file"`
	Installed bool   `json:"installed"`
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
		st := Status{AgentID: ad.ID(), AgentName: a.Name, Short: a.Short, File: host.ProviderFile(), Installed: a.Installed}
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
	p, err := s.Profile(name)
	if err != nil {
		return err
	}
	return s.each(agentID, func(id string, host agent.ProviderHost) error {
		return host.ApplyProvider(p, s.backupsDir)
	})
}

// Clear undoes what lazyagents applied, keeping the rest of the file.
func (s *Service) Clear(agentID string) error {
	return s.each(agentID, func(id string, host agent.ProviderHost) error {
		return host.ClearProvider(s.backupsDir)
	})
}

// each runs fn on the given agent, or on every installed agent that supports
// providers. One agent failing does not stop the others; errors are joined.
func (s *Service) each(agentID string, fn func(id string, host agent.ProviderHost) error) error {
	var errs []string
	var agents []agent.Agent
	if agentID == "" {
		agents = s.detectAll() // only "apply to all" needs to know who is installed
	}
	found := false
	for _, ad := range s.adapters {
		if agentID != "" && ad.ID() != agentID {
			continue
		}
		host, ok := ad.(agent.ProviderHost)
		if !ok {
			if agentID != "" {
				return fmt.Errorf("%s does not support switching providers", ad.ID())
			}
			continue
		}
		if agentID == "" && !findAgent(agents, ad.ID()).Installed {
			continue
		}
		found = true
		if err := fn(ad.ID(), host); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ad.ID(), err))
		}
	}
	switch {
	case !found && agentID != "":
		return fmt.Errorf("agent %q does not exist", agentID)
	case !found:
		return fmt.Errorf("no installed agent supports switching providers")
	case len(errs) > 0:
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
