package recovery

import (
	"os"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent/agenttest"
)

func TestMain(m *testing.M) {
	agenttest.ClearOverrides()
	os.Exit(m.Run())
}
