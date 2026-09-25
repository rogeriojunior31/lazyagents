package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// allAgents means "every installed agent that supports skills". Only the
// legacy (flat list) migration produces it; SaveProfile always writes concrete
// ids, so a migrated profile becomes concrete on its next save.
const allAgents = "*"

// ProfileSpec maps each skill (folder name) to the agents it should be
// enabled in: the body of a new-format profile.
type ProfileSpec map[string][]string

// ProfileChange is, for one skill, the agents applying the profile will enable
// (Add) and disable (Remove); the TUI shows it as a diff.
type ProfileChange struct {
	Skill  string
	Add    []string
	Remove []string
}

// readProfilesRaw reads the file as map[string]json.RawMessage so unknown
// fields survive the round-trip; a missing file is ok. Each profile decodes
// as the new format ({skill: [agentIDs]}) or the legacy one ([skills] →
// {skill: ["*"]}).
func (s *Service) readProfilesRaw() (map[string]json.RawMessage, map[string]ProfileSpec, error) {
	raw := make(map[string]json.RawMessage)
	data, err := os.ReadFile(s.paths.ProfilesPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("reading profiles: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, nil, fmt.Errorf("parsing profiles: %w", err)
		}
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	profiles := make(map[string]ProfileSpec)
	if pRaw, ok := raw["profiles"]; ok {
		perProfile := make(map[string]json.RawMessage)
		if err := json.Unmarshal(pRaw, &perProfile); err != nil {
			return nil, nil, fmt.Errorf("parsing profile list: %w", err)
		}
		for name, body := range perProfile {
			spec, err := decodeProfile(body)
			if err != nil {
				return nil, nil, fmt.Errorf("parsing profile %q: %w", name, err)
			}
			profiles[name] = spec
		}
	}
	return raw, profiles, nil
}

// decodeProfile accepts the new format (object) or the legacy one (skill array).
func decodeProfile(body json.RawMessage) (ProfileSpec, error) {
	var spec ProfileSpec
	if err := json.Unmarshal(body, &spec); err == nil {
		if spec == nil {
			spec = ProfileSpec{}
		}
		return spec, nil
	}
	var legacy []string
	if err := json.Unmarshal(body, &legacy); err != nil {
		return nil, err
	}
	spec = make(ProfileSpec, len(legacy))
	for _, sk := range legacy {
		if sk != "" {
			spec[sk] = []string{allAgents}
		}
	}
	return spec, nil
}

func (s *Service) writeProfiles(raw map[string]json.RawMessage, profiles map[string]ProfileSpec) error {
	pData, err := json.Marshal(profiles)
	if err != nil {
		return err
	}
	raw["profiles"] = pData
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(s.paths.ProfilesPath(), data, 0o644)
}

// ListProfiles returns the profile names in alphabetical order.
func (s *Service) ListProfiles() ([]string, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetProfile returns a profile's spec (skill → agents).
func (s *Service) GetProfile(name string) (ProfileSpec, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	spec, ok := profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	return spec, nil
}

// BuildProfileSpec snapshots the matrix: per library skill, the agents where it
// is enabled by our own symlink (Managed). Local and "echo" activations (seen
// only through another agent's shared dir, e.g. opencode reading
// ~/.claude/skills) are skipped: Apply cannot control them per agent.
func BuildProfileSpec(skills []Skill, agents []agent.Agent) ProfileSpec {
	spec := make(ProfileSpec)
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		var on []string
		for _, ag := range agents {
			if !ag.Installed || !ag.SupportsSkills() {
				continue
			}
			if sk.States[ag.ID].Managed {
				on = append(on, ag.ID)
			}
		}
		if len(on) > 0 {
			sort.Strings(on)
			spec[sk.Dir] = on
		}
	}
	return spec
}

// SaveProfile saves (or overwrites) the profile. Agent lists are deduplicated
// and sorted; empty ones are dropped.
func (s *Service) SaveProfile(name string, spec ProfileSpec) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	raw, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	clean := make(ProfileSpec, len(spec))
	for skill, agents := range spec {
		if skill == "" {
			continue
		}
		seen := make(map[string]bool, len(agents))
		list := make([]string, 0, len(agents))
		for _, a := range agents {
			if a != "" && !seen[a] {
				seen[a] = true
				list = append(list, a)
			}
		}
		if len(list) == 0 {
			continue
		}
		sort.Strings(list)
		clean[skill] = list
	}
	profiles[name] = clean
	return s.writeProfiles(raw, profiles)
}

// DeleteProfile removes the profile; deleting a missing one is not an error.
func (s *Service) DeleteProfile(name string) error {
	raw, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	delete(profiles, name)
	return s.writeProfiles(raw, profiles)
}

// resolveTargets turns the profile spec into the wanted agents per skill
// (expanding "*") and checks every referenced skill is in the library.
func resolveTargets(spec ProfileSpec, byDir map[string]Skill, skillAgents []agent.Agent) (map[string]map[string]bool, error) {
	var missing []string
	for w := range spec {
		if sk, found := byDir[w]; !found || !sk.InLibrary {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("profile refers to missing skills: %s", strings.Join(missing, ", "))
	}
	targets := make(map[string]map[string]bool, len(spec))
	for skill, agents := range spec {
		set := make(map[string]bool, len(agents))
		for _, a := range agents {
			if a == allAgents {
				for _, ag := range skillAgents {
					set[ag.ID] = true
				}
				continue
			}
			set[a] = true
		}
		targets[skill] = set
	}
	return targets, nil
}

// skillCapableAgents returns only the installed agents that support skills.
func skillCapableAgents(agents []agent.Agent) []agent.Agent {
	out := make([]agent.Agent, 0, len(agents))
	for _, ag := range agents {
		if ag.Installed && ag.SupportsSkills() {
			out = append(out, ag)
		}
	}
	return out
}

// ApplyProfile restores the profile matrix: each library skill ends up enabled
// exactly in the agents the profile asks for. Local skills are never touched.
// Idempotent; fails if the profile names missing skills.
func (s *Service) ApplyProfile(name string, agents []agent.Agent) error {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	spec, ok := profiles[name]
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	skills, err := s.Scan(agents)
	if err != nil {
		return fmt.Errorf("applying profile %q: %w", name, err)
	}
	byDir := make(map[string]Skill, len(skills))
	for _, sk := range skills {
		byDir[sk.Dir] = sk
	}

	capable := skillCapableAgents(agents)
	targets, err := resolveTargets(spec, byDir, capable)
	if err != nil {
		return fmt.Errorf("applying profile %q: %w", name, err)
	}

	var errs []error
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		want := targets[sk.Dir] // nil when the skill is not in the profile → disabled everywhere
		for _, ag := range capable {
			st := sk.States[ag.ID]
			switch {
			case want[ag.ID]:
				// Enable is a no-op when the agent already sees the skill (even via a
				// shared dir); it only symlinks when the agent does not.
				if err := s.Enable(sk, ag); err != nil {
					errs = append(errs, err)
				}
			case st.Managed:
				// only remove what we control: our symlink in the agent's dir; local
				// or echo activations (Via another agent) stay.
				if err := s.Disable(sk, ag); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
}

// DiffProfile compares the profile target with the current matrix and returns,
// per skill sorted by name, the agents to enable (Add) and disable (Remove).
// Local skills never appear in Remove.
func (s *Service) DiffProfile(name string, agents []agent.Agent) ([]ProfileChange, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	spec, ok := profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}

	skills, err := s.Scan(agents)
	if err != nil {
		return nil, fmt.Errorf("comparing profile %q: %w", name, err)
	}
	byDir := make(map[string]Skill, len(skills))
	for _, sk := range skills {
		byDir[sk.Dir] = sk
	}

	capable := skillCapableAgents(agents)
	targets, err := resolveTargets(spec, byDir, capable)
	if err != nil {
		return nil, fmt.Errorf("comparing profile %q: %w", name, err)
	}

	var changes []ProfileChange
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		want := targets[sk.Dir]
		var add, remove []string
		for _, ag := range capable {
			st := sk.States[ag.ID]
			if want[ag.ID] {
				if !st.On {
					add = append(add, ag.ID)
				}
			} else if st.Managed {
				// only what Apply can actually disable goes into the diff
				remove = append(remove, ag.ID)
			}
		}
		if len(add) > 0 || len(remove) > 0 {
			sort.Strings(add)
			sort.Strings(remove)
			changes = append(changes, ProfileChange{Skill: sk.Name, Add: add, Remove: remove})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return strings.ToLower(changes[i].Skill) < strings.ToLower(changes[j].Skill)
	})
	return changes, nil
}
