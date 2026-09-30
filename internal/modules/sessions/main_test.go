package sessions

import (
	"os"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent/agenttest"
)

// TestMain keeps the adapters these tests build away from the developer's
// real agent files (CODEX_HOME…).
func TestMain(m *testing.M) {
	agenttest.ClearOverrides()
	os.Exit(m.Run())
}
