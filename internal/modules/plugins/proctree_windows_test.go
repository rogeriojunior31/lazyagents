//go:build windows

package plugins

import (
	"os/exec"
	"strconv"
	"syscall"
)

// gone waits up to 5 s for pid to exit; a pid that cannot be opened is gone.
func gone(pid int) bool {
	const synchronize = 0x00100000
	h, err := syscall.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		return true
	}
	defer syscall.CloseHandle(h)
	ev, _ := syscall.WaitForSingleObject(h, 5000)
	return ev == syscall.WAIT_OBJECT_0
}

func kill(pid int) { _ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run() }
