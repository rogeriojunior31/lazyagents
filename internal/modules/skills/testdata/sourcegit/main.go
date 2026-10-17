// Command sourcegit is a cancellable Git fixture with a child process.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	root := os.Getenv("SOURCEGIT_FIXTURE")
	if len(os.Args) > 1 && os.Args[1] == "child" {
		for {
			_ = os.WriteFile(filepath.Join(root, "heartbeat"), []byte(time.Now().String()), 0o600)
			time.Sleep(20 * time.Millisecond)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "check-ref-format" {
		return
	}
	self, _ := os.Executable()
	child := exec.Command(self, "child")
	if err := child.Start(); err != nil {
		panic(err)
	}
	_ = os.WriteFile(filepath.Join(root, "pid"), fmt.Appendf(nil, "%d", child.Process.Pid), 0o600)
	_ = os.WriteFile(filepath.Join(root, "staging"), []byte(os.Args[len(os.Args)-1]), 0o600)
	time.Sleep(time.Minute)
}
