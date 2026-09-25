package hooks

// loadedMsg carries the library and each agent's state.
type loadedMsg struct {
	lib      []Hook
	problems []string
	statuses []Status
}

// doneMsg is the result of a write (enable, disable, delete).
type doneMsg struct {
	text string
	err  bool
}

// libraryMsg is the light boot read: the library only, for the counter.
type libraryMsg struct{ lib []Hook }
