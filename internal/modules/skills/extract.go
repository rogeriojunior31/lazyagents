package skills

import (
	"fmt"
	"io"
)

// Limits on what a zip from outside may unpack, so a small file cannot fill
// the disk. lazyagents' own backups are written with no such limit, so their
// restore keeps only the per-entry one. Variables so tests can lower them.
var (
	maxArchiveFiles       = 10_000
	maxEntryBytes   int64 = 64 << 20
	maxArchiveBytes int64 = 512 << 20
)

// extractBudget counts one archive's entries and bytes; a zero cap is no cap.
type extractBudget struct {
	maxFiles           int
	maxEntry, maxTotal int64
	files              int
	total              int64
}

// installBudget is for a zip from outside: every limit applies.
func installBudget() *extractBudget {
	return &extractBudget{maxFiles: maxArchiveFiles, maxEntry: maxEntryBytes, maxTotal: maxArchiveBytes}
}

// restoreBudget is for lazyagents' own backups: only the per-entry limit, which
// keeps one entry in memory bounded.
func restoreBudget() *extractBudget { return &extractBudget{maxEntry: maxEntryBytes} }

// entry counts one entry of any kind (file or dir).
func (b *extractBudget) entry(name string) error {
	if b.files++; b.maxFiles > 0 && b.files > b.maxFiles {
		return fmt.Errorf("archive has more than %d entries (stopped at %s)", b.maxFiles, name)
	}
	return nil
}

// read reads a file entry within the per-entry and total limits.
func (b *extractBudget) read(name string, r io.Reader) ([]byte, error) {
	limit := b.maxEntry
	if b.maxTotal > 0 {
		limit = min(limit, b.maxTotal-b.total)
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("extracting %s: %w", name, err)
	}
	if n := int64(len(data)); n > limit {
		if limit < b.maxEntry {
			return nil, fmt.Errorf("archive exceeds %d MB uncompressed (stopped at %s)", b.maxTotal>>20, name)
		}
		return nil, fmt.Errorf("entry %s exceeds %d MB", name, b.maxEntry>>20)
	}
	b.total += int64(len(data))
	return data, nil
}
