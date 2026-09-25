package agent

// LiveChecker is implemented by adapters that can tell whether a session is
// running now. Optional (not every agent supports it): resolve by type assertion.
type LiveChecker interface {
	// IsLive reports whether an agent process has the session open. false also
	// means "cannot tell".
	IsLive(s Session) bool
}
