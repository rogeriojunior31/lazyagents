package agent

import (
	"bytes"
	"io"
	"os"
	"strings"
)

// TranscriptProber is implemented by adapters whose transcript is text on disk:
// it tells, without decoding, whether the file MAY contain query. false is
// certain; true must be confirmed with Transcript (raw text has JSON keys and
// escapes). Optional, by type assertion.
type TranscriptProber interface {
	MayContain(s Session, query string) bool
}

const probeChunk = 1 << 20

// fileMayContain looks for query, case-insensitively, in the raw bytes of path.
// Only printable ASCII without quotes or backslashes can be matched raw (JSON
// may escape the rest: \u00e9, \"); anything else returns true.
func fileMayContain(path, query string) bool {
	if strings.ContainsAny(query, "\"\\") || strings.ContainsFunc(query, func(r rune) bool { return r < ' ' || r > '~' }) {
		return true
	}
	needle := bytes.ToLower([]byte(query))
	if len(needle) == 0 {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return true // unreadable: Transcript reports the error
	}
	defer f.Close()
	buf := make([]byte, probeChunk+len(needle))
	keep := 0 // tail of the previous chunk, for matches across chunks
	for {
		n, err := io.ReadFull(f, buf[keep:])
		if n > 0 && bytes.Contains(bytes.ToLower(buf[:keep+n]), needle) {
			return true
		}
		if err != nil {
			return false
		}
		keep = min(len(needle)-1, keep+n)
		copy(buf, buf[len(buf)-keep:])
	}
}

func (c *Claude) MayContain(s Session, query string) bool { return fileMayContain(s.Path, query) }
func (c *Codex) MayContain(s Session, query string) bool  { return fileMayContain(s.Path, query) }
