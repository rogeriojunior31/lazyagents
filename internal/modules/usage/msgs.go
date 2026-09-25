package usage

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// statusMsg carries each agent's limit windows (possibly from cache).
type statusMsg struct {
	statuses []Status
}

// eventsMsg carries usage events aggregated from transcripts.
type eventsMsg struct {
	events []agent.UsageEvent
}
