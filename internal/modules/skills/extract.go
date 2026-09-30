package skills

import (
	"fmt"
	"io"
)

// Limits on what one archive (a zip install, a tar.gz backup) may unpack, so
// a small file cannot fill the disk. Variables so tests can lower them.
var (
	maxArchiveFiles       = 10_000
	maxEntryBytes   int64 = 64 << 20
	maxArchiveBytes int64 = 512 << 20
)

// extractBudget counts one archive's entries and bytes against those limits.
type extractBudget struct {
	files int
	total int64
}

// entry counts one entry of any kind (file or dir).
func (b *extractBudget) entry(name string) error {
	if b.files++; b.files > maxArchiveFiles {
		return fmt.Errorf("archive has more than %d entries (stopped at %s)", maxArchiveFiles, name)
	}
	return nil
}

// read reads a file entry within the per-entry and total limits.
func (b *extractBudget) read(name string, r io.Reader) ([]byte, error) {
	limit := min(maxEntryBytes, maxArchiveBytes-b.total)
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("extracting %s: %w", name, err)
	}
	if n := int64(len(data)); n > limit {
		if limit < maxEntryBytes {
			return nil, fmt.Errorf("archive exceeds %d MB uncompressed (stopped at %s)", maxArchiveBytes>>20, name)
		}
		return nil, fmt.Errorf("entry %s exceeds %d MB", name, maxEntryBytes>>20)
	}
	b.total += int64(len(data))
	return data, nil
}
