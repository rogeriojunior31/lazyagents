package providers

import (
	"fmt"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Preparation contains destinations and requirements, never native values or tokens.
type Preparation struct {
	Profile string           `json:"profile"`
	Targets []PreparedTarget `json:"targets"`
	Error   string           `json:"error,omitempty"`
}

type PreparedTarget struct {
	ID string `json:"agent"`
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
	plan, _ := s.prepare(p, nil)
	return plan
}

func (s *Service) prepare(p agent.ProviderProfile, ids []string) (Preparation, core.OperationResult) {
	metadata := map[string]agent.ProviderValidation{}
	result := s.each(ids, func(id string, host agent.ProviderHost) error {
		validator, ok := host.(agent.ProviderValidator)
		if !ok {
			return fmt.Errorf("provider validation is unavailable for this agent")
		}
		v, err := validator.ValidateProvider(p)
		metadata[id] = v
		return err
	})
	plan := Preparation{Profile: p.Name, Targets: make([]PreparedTarget, 0, len(result.Targets))}
	if result.Err != nil {
		plan.Error = result.Err.Error()
	}
	for _, t := range result.Targets {
		target := PreparedTarget{ID: t.ID, ProviderValidation: metadata[t.ID], Skipped: t.Outcome == core.Skipped}
		if t.Outcome != core.Succeeded {
			target.Error = t.Detail
		}
		plan.Targets = append(plan.Targets, target)
	}
	return plan, result
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
