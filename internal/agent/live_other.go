//go:build !linux

package agent

import (
	"os/exec"
	"path/filepath"
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
	// lsof prints physical paths (macOS: /var is /private/var), so match on the
	// resolved path and report the caller's.
	want := make(map[string]string, len(paths))
	for _, p := range paths {
		want[p] = p
		if r, err := filepath.EvalSymlinks(p); err == nil {
			want[r] = p
		}
	}
	for _, line := range strings.Split(string(out), "\n") {
		if n, ok := strings.CutPrefix(line, "n"); ok {
			if p, ok := want[n]; ok {
				live[p] = true
			}
		}
	}
	return live
}
