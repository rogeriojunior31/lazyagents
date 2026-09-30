//go:build !windows

package plugins

import (
	"syscall"
	"time"
)

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

func kill(pid int) { _ = syscall.Kill(pid, syscall.SIGKILL) }
