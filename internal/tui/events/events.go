// Package events holds the Bubble Tea messages exchanged BETWEEN TUI modules.
// A message read by a module other than its producer lives here; the rest stay
// unexported in the module. The root broadcasts every non-input message.
package events

import "github.com/rogeriojunior31/lazyagents/internal/agent"

// AgentsDetected publishes one completed detection to every tab. Revisions
// prevent late boot/scan results from replacing evidence from a refresh.
type AgentsDetected struct {
	Agents   []agent.Agent
	Revision uint64
}

// SkillsScanned is sent after each skills scan. It carries only the aggregate
// other tabs need: events is framework and knows no module types.
type SkillsScanned struct {
	Revision      uint64
	ActiveByAgent map[string]int // enabled skills per agent id
	Total         int            // skills in the library
	Err           error
}

// SessionsLoaded is sent after each sessions load. Request orders loads within
// a detection revision so an older scan cannot replace a newer result.
type SessionsLoaded struct {
	Request   uint64
	Revision  uint64
	LocalOnly bool // detection refresh must not trigger usage/network queries
	Sessions  []agent.Session
	Err       error
}

// Reload asks the active module to reload its data (:reload).
type Reload struct{}

// AgentConfigChanged invalidates cached provider/hook state after recovery.
type AgentConfigChanged struct{ AgentID string }

// TabActivated marks a tab as visible. Expensive modules (network, transcript
// scans) load on it, never at boot.
type TabActivated struct {
	ID string // module.Module.ID() of the activated tab
}

// OpenTab requests navigation to a visible module without coupling features.
type OpenTab struct{ ID string }
