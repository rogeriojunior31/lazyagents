// Package agent is the ONLY place that knows the paths and file formats of the
// AI coding agents. One adapter per agent; registry.go builds the list.
package agent

import "time"

// Agent is a coding agent as detected (or not) on this machine.
type Agent struct {
	ID         string   // stable id, e.g. "claude-code"
	Name       string   // display name, e.g. "Claude Code"
	Short      string   // one letter for the TUI matrix, e.g. "C"
	Installed  bool     // binary in PATH and/or config dir present
	Version    string   // --version output, if any
	ManagedDir string   // the agent's own skills dir, where lazyagents enables skills ("" = unsupported)
	SharedDir  string   // cross-agent skills dir it also reads (~/.agents/skills); used only when every reader has the skill
	ReadDirs   []string // ALL skill dirs the agent reads (includes ManagedDir and SharedDir)
	Detail     string   // how it was detected / notes
}

// DetailNotInstalled is the Detail of an agent that was not found.
const DetailNotInstalled = "not installed"

// SupportsSkills reports whether the agent has a manageable skills dir.
func (a Agent) SupportsSkills() bool { return a.ManagedDir != "" }

// Session is an agent conversation, normalized across agents.
type Session struct {
	AgentID   string
	AgentName string
	ID        string // id used to resume
	Path      string // source file or record
	CWD       string // working dir ("" if unknown)
	Title     string // first prompt or conversation title
	MTime     time.Time
	Alias     string // lazyagents alias (set by the sessions service, never by adapters)
}

// Adapter is implemented by every supported agent.
type Adapter interface {
	// ID returns the stable agent id (cheap, no I/O).
	ID() string
	Detect() Agent
	ListSessions() ([]Session, error)
	// ResumeCmd returns the argv that resumes s and the dir to run it in.
	// ok=false when the agent cannot resume from its CLI.
	ResumeCmd(s Session) (argv []string, dir string, ok bool)
	Transcript(s Session) ([]Entry, error)
	// DeleteSession backs the session up into backupsDir and removes it.
	DeleteSession(s Session, backupsDir string) error
}
