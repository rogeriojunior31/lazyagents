//go:build !windows

package plugins

import (
	"errors"
	"os/exec"
	"syscall"
)

// ownGroup makes cmd lead a new process group, so stopping it also stops what
// it started. Only for commands off the terminal: a background group that
// reads the TTY is stopped by the kernel (SIGTTIN).
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// reapGroup kills whatever is left of the group once its leader exited: a
// child the plugin started must not outlive it.
func reapGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// startedGroup has nothing to do on Unix: the group exists from the start.
func startedGroup(*exec.Cmd) {}
