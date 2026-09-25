// Package events holds the Bubble Tea messages exchanged BETWEEN TUI modules.
// A message read by a module other than its producer lives here; the rest stay
// unexported in the module. The root broadcasts every non-input message.
package events

import "github.com/rogeriojunior31/lazyagents/internal/agent"

// AgentsDetected is sent by the root after agent.DetectAll; it triggers the
// first skills scan and sessions load.
type AgentsDetected struct {
	Agents []agent.Agent
}

// SkillsScanned is sent after each skills scan. It carries only the aggregate
// other tabs need: events is framework and knows no module types.
type SkillsScanned struct {
	ActiveByAgent map[string]int // enabled skills per agent id
	Total         int            // skills in the library
	Err           error
}

// SessionsLoaded is sent after each sessions load.
type SessionsLoaded struct {
	Sessions []agent.Session
	Err      error
}

// Reload asks the active module to reload its data (:reload).
type Reload struct{}

// TabActivated marks a tab as visible. Expensive modules (network, transcript
// scans) load on it, never at boot.
type TabActivated struct {
	ID string // module.Module.ID() of the activated tab
}
