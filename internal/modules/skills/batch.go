package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

type batchAction int

const (
	batchEnable batchAction = iota
	batchDisable
	batchUpdate
	batchRemove
)

func (a batchAction) String() string {
	return [...]string{"Enable", "Disable", "Update", "Remove"}[a]
}

// A plan owns the reviewed rows and destinations. Retries may narrow it, never
// resolve a new UI selection or follow a changed update source.
type skillBatch struct {
	action batchAction
	skills []Skill
	agents []agent.Agent
	ids    []string
}

func batchTarget(dir, id string) string { return dir + " / " + id }

func (s *Service) runBatch(plan skillBatch, retry []string) core.OperationResult {
	result := core.OperationResult{}
	for _, saved := range plan.skills {
		ids := []string{"library"}
		if plan.action <= batchDisable {
			ids = plan.ids
		}
		ids = slices.DeleteFunc(slices.Clone(ids), func(id string) bool {
			return retry != nil && !slices.Contains(retry, batchTarget(saved.Dir, id))
		})
		if len(ids) == 0 {
			continue
		}
		add := func(outcome core.TargetOutcome, detail, state string) {
			for _, id := range ids {
				result.Targets = append(result.Targets, core.TargetResult{
					ID: batchTarget(saved.Dir, id), Outcome: outcome, Detail: detail, State: state,
				})
			}
		}
		// ponytail: reuse the scan/placement operations per skill; share the scan
		// if large selected batches prove slow.
		current, err := s.Scan(plan.agents)
		if err != nil {
			add(core.Failed, err.Error(), "unreadable")
			continue
		}
		sk, found := findSkill(saved.Dir, current)
		if !found || !sk.InLibrary {
			add(core.Skipped, "not in the library; local or foreign content preserved", "library entry absent")
			continue
		}
		if !saved.InLibrary || sk.Path != saved.Path {
			add(core.Failed, "skill location changed since review; select it again", "not applied")
			continue
		}
		info, err := os.Lstat(sk.Path)
		if err != nil {
			add(core.Failed, err.Error(), "unreadable")
			continue
		}
		if !info.IsDir() {
			add(core.Skipped, "library entry is a link; external content preserved", "unchanged")
			continue
		}
		if plan.action <= batchDisable {
			r := s.ToggleResult(sk.Dir, plan.action == batchEnable, plan.agents, ids)
			for _, t := range r.Targets {
				t.ID = batchTarget(sk.Dir, t.ID)
				result.Targets = append(result.Targets, t)
			}
			if len(r.Targets) == 0 && r.Err != nil {
				add(core.Failed, r.Err.Error(), "not applied")
			}
			if r.Err != nil {
				result.Err = errors.Join(result.Err, fmt.Errorf("%s: %w", sk.Dir, r.Err))
			}
			continue
		}
		if plan.action == batchUpdate {
			if sk.Origin == nil || sk.Origin.Type != "git" {
				add(core.Skipped, ErrNoGitOrigin.Error(), "unchanged")
				continue
			}
			if saved.Origin == nil || saved.Origin.Type != sk.Origin.Type || saved.Origin.Source != sk.Origin.Source || saved.Origin.Sub != sk.Origin.Sub || saved.Origin.Ref != sk.Origin.Ref || saved.Origin.ExactPath != sk.Origin.ExactPath {
				add(core.Failed, "update source changed since review; select it again", "not applied")
				continue
			}
			hash, err := hashDir(sk.Path)
			if err != nil {
				add(core.Failed, err.Error(), "unreadable")
				continue
			}
			if sk.Origin.Hash == "" || hash != sk.Origin.Hash {
				add(core.Skipped, "local edits or missing baseline; review with the single-skill update", "unchanged")
				continue
			}
			err = s.Update(sk)
		} else {
			err = s.Remove(sk, plan.agents)
		}
		state := "library entry present"
		if _, statErr := os.Lstat(filepath.Join(s.paths.LibraryDir(), sk.Dir)); os.IsNotExist(statErr) {
			state = "library entry absent"
		} else if statErr != nil {
			state = "unreadable"
		}
		if err != nil {
			add(core.Failed, err.Error(), state)
		} else {
			add(core.Succeeded, "", state)
		}
	}
	return result
}
