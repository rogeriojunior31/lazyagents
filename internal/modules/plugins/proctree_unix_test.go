//go:build !windows

package plugins

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins/plugintest"
)

// childPID waits for the fake plugin to report the child it started.
func childPID(t *testing.T, file string) int {
	t.Helper()
	for range 100 {
		if data, err := os.ReadFile(file); err == nil && len(data) > 0 {
			pid, _ := strconv.Atoi(string(data))
			return pid
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the plugin never reported its child")
	return 0
}

// gone waits for pid to stop existing.
func gone(pid int) bool {
	for range 100 {
		if syscall.Kill(pid, 0) != nil {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// Closing a plugin stops the processes it started, not only the plugin.
func TestCloseStopsProcessTree(t *testing.T) {
	s := newGoService(t)
	s.Handshake = 3 * time.Second // a fresh binary can be slow to start
	plugintest.Install(t, s.Dir, "spawner")
	pidfile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("FAKEPLUGIN_MODE", "spawn")
	t.Setenv("FAKEPLUGIN_PIDFILE", pidfile)
	pls, _ := s.List()
	p, err := s.Start(pls[0], Msg{})
	if err != nil {
		t.Fatal(err)
	}
	child := childPID(t, pidfile)
	if syscall.Kill(child, 0) != nil {
		t.Fatal("the child should be running while the plugin is")
	}
	_ = p.Close()
	if !gone(child) {
		_ = syscall.Kill(child, syscall.SIGKILL)
		t.Fatal("the plugin's child outlived it")
	}
}
