package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// A file this process has open counts as live, even when asked through a
// symlinked path; a closed one does not.
func TestLiveOpenFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("outside Linux this needs lsof installed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "via-link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	viaLink := filepath.Join(link, "s.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	live := liveOpenFiles([]string{path, viaLink})
	if !live[path] || !live[viaLink] {
		t.Errorf("open file not detected: %v", live)
	}
	f.Close()
	if live := liveOpenFiles([]string{path}); live[path] {
		t.Error("closed file still counts as live")
	}
}
