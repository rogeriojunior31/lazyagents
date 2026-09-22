package sessions

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/session"
)

type resumeDoneMsg struct{ err error }

type searchDoneMsg struct {
	query   string
	matches []session.Match
	err     error
}

type transcriptMsg struct {
	session agent.Session
	title   string
	entries []agent.Entry
	err     error
}

// exportDoneMsg é o resultado de exportar o transcript aberto pra Markdown
// (tecla x no sessModeDoc, M8.A5).
type exportDoneMsg struct {
	path string
	err  error
}

type usageMsg struct {
	id    string
	usage agent.Usage
	ok    bool
}

type deleteSessionsMsg struct {
	deleted int
	failed  int
	errs    []error
}
