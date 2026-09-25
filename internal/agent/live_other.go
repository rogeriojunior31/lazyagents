//go:build !linux

package agent

import (
	"os/exec"
	"strings"
)

// liveOpenFiles returns which of paths some process has open, with a single
// lsof call for all of them (never one per session). Best-effort: never an error.
func liveOpenFiles(paths []string) map[string]bool {
	live := make(map[string]bool)
	if len(paths) == 0 {
		return live
	}
	// -F n: one "n<path>" line per open file, no columns to parse
	out, _ := exec.Command("lsof", append([]string{"-F", "n", "--"}, paths...)...).Output()
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "n"); ok && want[p] {
			live[p] = true
		}
	}
	return live
}
