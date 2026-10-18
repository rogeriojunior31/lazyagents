package hooks

import "github.com/rogeriojunior31/lazyagents/internal/core"

// loadedMsg carries the library and each agent's state.
type loadedMsg struct {
	request  uint64
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

type bulkMsg struct {
	command bool
	index   int
	name    string
	enable  bool
	result  core.OperationResult
}
