package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Arquivo aberto por este processo conta como vivo, inclusive pedido por um
// caminho com symlink; fechado, não.
func TestLiveOpenFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fora do Linux depende do lsof instalado")
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
		t.Errorf("aberto não detectado: %v", live)
	}
	f.Close()
	if live := liveOpenFiles([]string{path}); live[path] {
		t.Error("fechado ainda conta como vivo")
	}
}
