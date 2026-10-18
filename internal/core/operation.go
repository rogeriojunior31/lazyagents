package core

import (
	"errors"
	"fmt"
)

type TargetOutcome string

const (
	Succeeded TargetOutcome = "succeeded"
	Failed    TargetOutcome = "failed"
	Skipped   TargetOutcome = "skipped"
)

// TargetResult describes an attempt, not a transaction: a failed target may
// have changed partially. State is read back after the attempt, without secrets.
type TargetResult struct {
	ID      string
	Outcome TargetOutcome
	Detail  string
	State   string
}

type OperationResult struct {
	Targets        []TargetResult
	Err            error  // preflight or operation-wide failure
	HistoryWarning string // persistence failed after the attempt; never a retry target
}

func (r OperationResult) AsError() error {
	errs := []error{r.Err}
	for _, t := range r.Targets {
		if t.Outcome == Failed {
			errs = append(errs, fmt.Errorf("%s: %s", t.ID, t.Detail))
		}
	}
	return errors.Join(errs...)
}

// CompletionError includes journal failures for legacy error-only callers.
// Structured reports keep HistoryWarning separate so successes stay successful.
func (r OperationResult) CompletionError() error {
	if r.HistoryWarning != "" {
		return errors.Join(r.AsError(), errors.New(r.HistoryWarning))
	}
	return r.AsError()
}

// FailedIDs always returns a non-nil slice: an empty retry must never mean all.
func (r OperationResult) FailedIDs() []string {
	ids := []string{}
	for _, t := range r.Targets {
		if t.Outcome == Failed {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// TargetIDs translates the legacy empty agent argument to all targets (nil).
func TargetIDs(id string) []string {
	if id == "" {
		return nil
	}
	return []string{id}
}

// MergeRetry retains the outcomes of targets that were not retried.
func (r *OperationResult) MergeRetry(next OperationResult) {
	r.Err = next.Err
	r.HistoryWarning = next.HistoryWarning
	for _, t := range next.Targets {
		found := false
		for i := range r.Targets {
			if r.Targets[i].ID == t.ID {
				r.Targets[i], found = t, true
				break
			}
		}
		if !found {
			r.Targets = append(r.Targets, t)
		}
	}
}
