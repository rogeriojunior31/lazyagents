package usage

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// statusMsg carries each agent's limit windows (possibly from cache) and the
// agents billed per token, which include agents without limits (Pi).
type statusMsg struct {
	statuses []Status
	api      map[string]bool
}

// eventsMsg carries usage events aggregated from transcripts.
type eventsMsg struct {
	events []agent.UsageEvent
}
