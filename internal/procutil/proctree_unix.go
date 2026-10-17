//go:build !windows

package procutil

import (
	"errors"
	"os/exec"
	"syscall"
)

// OwnGroup makes cmd lead a new process group, so stopping it also stops what
// it started. Only for commands off the terminal: a background group that
// reads the TTY is stopped by the kernel (SIGTTIN).
func OwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// ReapGroup kills whatever is left of the group once its leader exited: a
// child the command started must not outlive it.
func ReapGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// StartedGroup has nothing to do on Unix: the group exists from the start.
func StartedGroup(*exec.Cmd) {}
