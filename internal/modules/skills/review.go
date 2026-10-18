package skills

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Callers pass concrete JSON-safe snapshots; sorted slices and JSON map keys
// make review identities independent of scan/map iteration order.
func reviewHash(snapshot any) string {
	data, _ := json.Marshal(snapshot)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (s *Service) updateReviewToken(checks []UpdateCheck) string {
	if len(checks) == 0 {
		return ""
	}
	type item struct {
		Dir                     string
		Origin                  *Origin
		Local, Remote, Revision string
		Status                  UpdateStatus
		Error                   string
	}
	items := make([]item, 0, len(checks))
	for _, c := range checks {
		if c.Err != nil && !updateIsSkipped(c.Err) {
			return ""
		}
		failure := ""
		if c.Err != nil {
			failure = c.Err.Error()
		}
		items = append(items, item{c.Skill.Dir, c.Skill.Origin, c.LocalHash, c.RemoteHash, c.Revision, c.Status, failure})
	}
	slices.SortFunc(items, func(a, b item) int { return strings.Compare(a.Dir, b.Dir) })
	return reviewHash(struct {
		Library string
		Items   []item
	}{s.paths.LibraryDir(), items})
}

type profileReview struct {
	spec    ProfileSpec
	skills  []Skill
	changes []ProfileChange
	token   string
}

func (s *Service) previewProfile(name string, agents []agent.Agent) (profileReview, error) {
	r := profileReview{}
	var err error
	r.spec, err = s.GetProfile(name)
	if err != nil {
		return r, err
	}
	r.skills, err = s.Scan(agents)
	if err != nil {
		return r, err
	}
	r.changes, err = diffProfile(r.spec, r.skills, agents)
	if err != nil {
		return r, err
	}
	type destination struct {
		ID              string
		Installed       bool
		Managed, Shared string
		Read            []string
	}
	dests := make([]destination, 0, len(agents))
	for _, a := range agents {
		dests = append(dests, destination{a.ID, a.Installed, a.ManagedDir, a.SharedDir, a.ReadDirs})
	}
	slices.SortFunc(dests, func(a, b destination) int { return strings.Compare(a.ID, b.ID) })
	skills := slices.Clone(r.skills)
	slices.SortFunc(skills, func(a, b Skill) int { return strings.Compare(a.Dir, b.Dir) })
	r.token = reviewHash(struct {
		Library, Name string
		Spec          ProfileSpec
		Skills        []Skill
		Agents        []destination
	}{s.paths.LibraryDir(), name, r.spec, skills, dests})
	return r, nil
}

func (s *Service) applyReviewedProfile(name, token string, agents []agent.Agent) core.OperationResult {
	r, err := s.previewProfile(name, agents)
	if err != nil {
		return core.OperationResult{Err: err}
	}
	if token == "" || token != r.token {
		return core.OperationResult{Err: fmt.Errorf("profile, inventory or destinations changed since preview; preview again")}
	}
	return s.applyProfileSnapshot(name, r.spec, r.skills, agents, nil)
}
