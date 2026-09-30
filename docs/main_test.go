package docs

import (
	"os"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// TestMain clears the agent CLIs' config variables (CODEX_HOME…): the adapters
// read them, and a test must never reach the developer's real agent files.
func TestMain(m *testing.M) {
	for _, o := range agent.ConfigOverrides {
		os.Unsetenv(o.Var)
	}
	os.Exit(m.Run())
}
