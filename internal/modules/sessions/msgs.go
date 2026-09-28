package sessions

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

type resumeDoneMsg struct{ err error }

// aliasDoneMsg ends SetAlias (key m).
type aliasDoneMsg struct {
	key   string // agent:id
	alias string
	err   error
}

type searchDoneMsg struct {
	query   string
	matches []Match
	err     error
}

type transcriptMsg struct {
	session agent.Session
	title   string
	entries []agent.Entry
	err     error
}

// exportDoneMsg is the result of exporting the open transcript (key x).
type exportDoneMsg struct {
	path string
	err  error
}

type usageMsg struct {
	id      string
	usage   agent.Usage
	ok      bool
	cost    float64
	hasCost bool // API-key account and a priced model
}

type deleteSessionsMsg struct {
	deleted int
	failed  int
	errs    []error
}
