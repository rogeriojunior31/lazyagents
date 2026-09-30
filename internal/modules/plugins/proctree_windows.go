//go:build windows

package plugins

import (
	"os/exec"
	"strconv"
	"syscall"
)

// ownGroup starts cmd in its own process group; stopping it kills the whole
// tree with taskkill /T while the leader still runs.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	cmd.Cancel = func() error {
		if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// reapGroup is a no-op on Windows: once the leader exited its children are no
// longer reachable as a tree (that needs a Job Object).
func reapGroup(*exec.Cmd) {}
