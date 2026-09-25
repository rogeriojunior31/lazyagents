package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileMayContain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	// "Needle" crosses the boundary of the first read chunk
	data := strings.Repeat("x", probeChunk-3) + `Needle in haystack`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		query string
		want  bool
	}{
		{"needle", true},   // case-insensitive, across chunks
		{"HAYSTACK", true}, // end of file
		{"missing", false},
		{`with "quotes"`, true}, // JSON would escape it: the full search decides
		{"açúcar", true},        // non-ASCII: same // check-english:allow
	}
	for _, tc := range cases {
		if got := fileMayContain(path, tc.query); got != tc.want {
			t.Errorf("fileMayContain(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
	if !fileMayContain(filepath.Join(t.TempDir(), "missing"), "x") {
		t.Error("an unreadable file must go on to Transcript, which reports the error")
	}
}
