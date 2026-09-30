// Package agenttest keeps tests away from the developer's agent config. Only
// tests import it.
package agenttest

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// ClearOverrides unsets the agent CLIs' config variables (agent.ConfigOverrides)
// so adapters built in tests never point at real files. Call it from TestMain.
// On Windows that includes LOCALAPPDATA, where `go build` (the fake plugin)
// finds its cache: GOCACHE is pinned to it first.
func ClearOverrides() {
	if runtime.GOOS == "windows" && os.Getenv("GOCACHE") == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			_ = os.Setenv("GOCACHE", filepath.Join(dir, "go-build"))
		}
	}
	for _, o := range agent.ConfigOverrides {
		_ = os.Unsetenv(o.Var)
	}
}
