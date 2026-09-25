package providers

import "github.com/rogeriojunior31/lazyagents/internal/agent"

// loadedMsg carries the profile library and each agent's state.
type loadedMsg struct {
	profiles []agent.ProviderProfile
	statuses []Status
	err      error
}

// doneMsg is the result of a write (apply, clear, delete).
type doneMsg struct {
	text string
	err  bool
}

// clearAllMsg comes from the palette: providers clear.
type clearAllMsg struct{}

// profilesMsg is the light boot read: only profiles, for the tab count.
type profilesMsg struct{ profiles []agent.ProviderProfile }

// savedMsg is the result of the profile form.
type savedMsg struct {
	name string
	err  error
}

// newProfileMsg opens the profile form (palette).
type newProfileMsg struct{}
