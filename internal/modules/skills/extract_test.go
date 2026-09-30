package skills

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// limits lowers the archive limits for one test.
func limits(t *testing.T, files int, entry, total int64) {
	t.Helper()
	f, e, a := maxArchiveFiles, maxEntryBytes, maxArchiveBytes
	maxArchiveFiles, maxEntryBytes, maxArchiveBytes = files, entry, total
	t.Cleanup(func() { maxArchiveFiles, maxEntryBytes, maxArchiveBytes = f, e, a })
}

func writeZip(t *testing.T, entries map[string]int) string {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, size := range entries {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(bytes.Repeat([]byte("x"), size))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.zip")
	writeFileT(t, path, buf.Bytes())
	return path
}

func writeFileT(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Many entries under the per-entry limit still stop at the total, and the
// failed extraction leaves nothing in the temp dir.
func TestExtractZipLimits(t *testing.T) {
	limits(t, 5, 100, 250)
	tmpdir := t.TempDir()
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} { // os.TempDir per OS
		t.Setenv(v, tmpdir)
	}
	entries := map[string]int{}
	for i := range 3 {
		entries[fmt.Sprintf("skill/f%d.md", i)] = 100
	}
	if _, err := extractZip(writeZip(t, entries)); err == nil || !strings.Contains(err.Error(), "uncompressed") {
		t.Errorf("total cap: err = %v", err)
	}
	many := map[string]int{}
	for i := range 6 {
		many[fmt.Sprintf("skill/f%d.md", i)] = 1
	}
	if _, err := extractZip(writeZip(t, many)); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Errorf("entry count cap: err = %v", err)
	}
	if left, _ := os.ReadDir(tmpdir); len(left) != 0 {
		t.Errorf("a failed extraction left %d item(s) in the temp dir", len(left))
	}
	dir, err := extractZip(writeZip(t, map[string]int{"skill/SKILL.md": 100, "skill/b.md": 100}))
	if err != nil {
		t.Fatalf("within limits: %v", err)
	}
	os.RemoveAll(dir)
}

func TestExtractTarGzLimits(t *testing.T) {
	limits(t, 5, 100, 250)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i := range 3 {
		data := bytes.Repeat([]byte("x"), 100)
		_ = tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("s/f%d", i), Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(data)
	}
	_ = tw.Close()
	_ = gz.Close()
	src := filepath.Join(t.TempDir(), "b.tar.gz")
	writeFileT(t, src, buf.Bytes())
	if err := extractTarGz(src, t.TempDir()); err == nil || !strings.Contains(err.Error(), "uncompressed") {
		t.Errorf("total cap: err = %v", err)
	}
}
