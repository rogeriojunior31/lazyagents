// Package procutil owns background commands and their child processes.
package procutil

import (
	"bytes"
	"os/exec"
	"time"
)

// Output captures bounded output and reaps children even when the leader exits.
func Output(cmd *exec.Cmd) ([]byte, error) {
	var out limitedOutput
	cmd.Stdout, cmd.Stderr = &out, &out
	err := Run(cmd)
	return out.Bytes(), err
}

// Run starts an owned background process and reaps its entire tree.
func Run(cmd *exec.Cmd) error {
	cmd.WaitDelay = time.Second
	OwnGroup(cmd)
	err := cmd.Start()
	if err == nil {
		StartedGroup(cmd)
		err = cmd.Wait()
	}
	ReapGroup(cmd)
	return err
}

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if left := (64 << 10) - b.Len(); left > 0 {
		_, _ = b.Buffer.Write(p[:min(left, n)])
	}
	return n, nil
}
