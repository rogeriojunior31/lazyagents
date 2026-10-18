package providers

import (
	"fmt"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Preparation contains destinations and requirements, never native values or tokens.
type Preparation struct {
	Operation string           `json:"operation"`
	Profile   string           `json:"profile"`
	Targets   []PreparedTarget `json:"targets"`
	Error     string           `json:"error,omitempty"`
}

type PreparedTarget struct {
	ID      string                `json:"agent"`
	Preview agent.ProviderPreview `json:"preview"`
	agent.ProviderValidation
	Error   string `json:"error,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// PrepareSaved validates the current saved profile without creating any files.
func (s *Service) PrepareSaved(name string, ids []string) Preparation {
	p, err := s.Profile(name)
	if err != nil {
		return Preparation{Profile: name, Error: err.Error()}
	}
	plan, _ := s.prepare(p, ids)
	return plan
}

// PrepareDraft shares the adapter validation with apply; saving a draft does not
// require compatibility with every installed agent.
func (s *Service) PrepareDraft(p agent.ProviderProfile) Preparation {
	plan, _ := s.prepareOperation(&p, nil, false)
	return plan
}

func (s *Service) prepare(p agent.ProviderProfile, ids []string) (Preparation, core.OperationResult) {
	return s.prepareOperation(&p, ids, true)
}

func (s *Service) PrepareClear(ids []string) Preparation {
	plan, _ := s.prepareOperation(nil, ids, true)
	return plan
}

func (s *Service) prepareOperation(p *agent.ProviderProfile, ids []string, preview bool) (Preparation, core.OperationResult) {
	metadata := map[string]agent.ProviderValidation{}
	previews := map[string]agent.ProviderPreview{}
	result := s.each(ids, func(id string, host agent.ProviderHost) error {
		if p != nil {
			validator, ok := host.(agent.ProviderValidator)
			if !ok {
				return fmt.Errorf("provider validation is unavailable for this agent")
			}
			v, err := validator.ValidateProvider(*p)
			metadata[id] = v
			if err != nil {
				return err
			}
		} else {
			metadata[id] = agent.ProviderValidation{Files: agent.ProviderFiles(host), Notes: []string{"Remove only managed provider overrides; previous or external settings may remain. Native login stays in the agent."}}
		}
		if preview {
			pe, ok := host.(agent.ProviderPreviewer)
			if !ok {
				return fmt.Errorf("provider previews are unavailable for this agent")
			}
			v, err := pe.PreviewProvider(p)
			previews[id] = v
			return err
		}
		return nil
	})
	plan := Preparation{Operation: "clear", Profile: "Agent defaults / previous settings", Targets: make([]PreparedTarget, 0, len(result.Targets))}
	if p != nil {
		plan.Operation, plan.Profile = "apply", p.Name
	}
	if result.Err != nil {
		plan.Error = result.Err.Error()
	}
	for _, t := range result.Targets {
		target := PreparedTarget{ID: t.ID, ProviderValidation: metadata[t.ID], Preview: previews[t.ID], Skipped: t.Outcome == core.Skipped}
		if t.Outcome != core.Succeeded {
			target.Error = t.Detail
		}
		plan.Targets = append(plan.Targets, target)
	}
	return plan, result
}

func reviewChanged(review Preparation, current Preparation, ids []string) bool {
	if review.Operation != current.Operation || review.Profile != current.Profile {
		return true
	}
	if ids == nil {
		ids = make([]string, 0)
		for _, t := range current.Targets {
			if !t.Skipped {
				ids = append(ids, t.ID)
			}
		}
	}
	for _, id := range ids {
		old, now := "", ""
		for _, t := range review.Targets {
			if t.ID == id {
				old = t.Preview.Revision
			}
		}
		for _, t := range current.Targets {
			if t.ID == id {
				now = t.Preview.Revision
			}
		}
		if old == "" || now == "" || old != now {
			return true
		}
	}
	return false
}

func failedPreparation(result core.OperationResult) core.OperationResult {
	if result.Err == nil {
		result.Err = fmt.Errorf("provider preparation failed before writing; no agent files were changed")
	}
	for i := range result.Targets {
		t := &result.Targets[i]
		if t.Outcome == core.Succeeded {
			t.Outcome, t.Detail = core.Failed, "not attempted: another selected agent failed preparation; fix the selection before applying"
		}
	}
	return result
}

func blockedReview(result core.OperationResult) core.OperationResult {
	result.Err = fmt.Errorf("provider settings or the saved profile changed since review; review the current preview before applying")
	for i := range result.Targets {
		if result.Targets[i].Outcome != core.Skipped {
			result.Targets[i].Outcome = core.Failed
			result.Targets[i].Detail = "not attempted: review changed; no files were written"
		}
	}
	return result
}

func (s *Service) ClearReviewed(ids []string, review Preparation) core.OperationResult {
	current, result := s.prepareOperation(nil, ids, true)
	if result.AsError() != nil {
		return failedPreparation(result)
	}
	if reviewChanged(review, current, ids) {
		return blockedReview(result)
	}
	selected := make([]string, 0)
	for _, t := range result.Targets {
		if t.Outcome == core.Succeeded {
			selected = append(selected, t.ID)
		}
	}
	cleared := s.ClearResult(selected)
	result.HistoryWarning = cleared.HistoryWarning
	for i := range result.Targets {
		for _, t := range cleared.Targets {
			if t.ID == result.Targets[i].ID {
				result.Targets[i] = t
			}
		}
	}
	return result
}

func (p Preparation) valid() bool {
	if p.Error != "" {
		return false
	}
	for _, t := range p.Targets {
		if !t.Skipped && t.Error != "" {
			return false
		}
	}
	return true
}
