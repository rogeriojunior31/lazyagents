package agent

import (
	"os"
	"path/filepath"
)

// liveOpenFiles returns which of paths some process has open. On Linux it reads
// /proc/<pid>/fd directly, avoiding lsof's ~130 ms startup; other users'
// processes are unreadable, which is fine (agents run as the user).
// Best-effort: never an error.
func liveOpenFiles(paths []string) map[string]bool {
	live := make(map[string]bool)
	if len(paths) == 0 {
		return live
	}
	want := make(map[string][]string, len(paths)) // real path → as requested
	for _, p := range paths {
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			real = p
		}
		want[real] = append(want[real], p)
	}
	procs, _ := os.ReadDir("/proc")
	for _, pr := range procs {
		if pr.Name()[0] < '0' || pr.Name()[0] > '9' {
			continue
		}
		dir := filepath.Join("/proc", pr.Name(), "fd")
		fds, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if target, err := os.Readlink(filepath.Join(dir, fd.Name())); err == nil {
				for _, p := range want[target] {
					live[p] = true
				}
			}
		}
	}
	return live
}
