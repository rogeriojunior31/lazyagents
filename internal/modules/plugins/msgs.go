package plugins

// Every message carries the plugin id: the root broadcasts async messages to
// every tab, and each proxy ignores the others'.

// frameMsg is a plugin message read from Proc.Events.
type frameMsg struct {
	id  string
	msg Msg
}

// exitMsg means Events closed: the plugin died or broke the protocol.
type exitMsg struct {
	id     string
	err    error
	stderr string
}

// commandMsg is a plugin palette entry picked by the user.
type commandMsg struct {
	id   string
	name string
}

// execDoneMsg is the end of an exec the plugin requested.
type execDoneMsg struct {
	id     string
	execID int
	code   int
	stdout string
	stderr string
	err    error
}
