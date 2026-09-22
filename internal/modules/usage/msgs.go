package usage

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// statusMsg traz as janelas de limite de cada agente (pode ter vindo do cache).
type statusMsg struct {
	statuses []Status
}

// eventsMsg traz os eventos de uso já agregados a partir dos transcripts.
type eventsMsg struct {
	events []agent.UsageEvent
}
