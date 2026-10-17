package cli

import (
	"fmt"
	"io"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// PrintOperation leaves existing stdout/JSON contracts intact; diagnostics and
// retry guidance go to stderr, including successful targets of a partial write.
func PrintOperation(w io.Writer, r core.OperationResult, recovery string) {
	for _, t := range r.Targets {
		fmt.Fprintf(w, "%s: %s", t.ID, t.Outcome)
		if t.Detail != "" {
			fmt.Fprintf(w, " — %s", t.Detail)
		}
		if t.State != "" {
			fmt.Fprintf(w, " (current: %s)", t.State)
		}
		fmt.Fprintln(w)
	}
	if r.Err != nil {
		fmt.Fprintln(w, "lazyagents:", r.Err)
	}
	if ids := r.FailedIDs(); len(ids) > 0 {
		fmt.Fprintln(w, "Changes may be partial. Successful targets are already applied.")
		for _, id := range ids {
			fmt.Fprintf(w, "Retry only %s with --agent %s after fixing the cause.\n", id, id)
		}
	}
	if r.AsError() != nil {
		fmt.Fprintln(w, "Recovery:", recovery)
	}
}
