// Package fsutil owns every disk write: WriteAtomic (tmp + rename), Backup and
// RotateBackups.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// backupTimeLayout is fixed-width so sorting names sorts backups by time.
const backupTimeLayout = "20060102T150405.000000000"

// WriteAtomic writes through a temp file in the same dir (rename does not
// cross filesystems) renamed over path, creating the parent (0700) if needed.
// On error the live file is untouched and the temp file is removed.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting temp file permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// Backup copies path into backupDir as "<basename>.<timestamp>", keeping the
// original mode (it may hold secrets), and returns the backup path. A missing
// path is a no-op returning ("", nil) and does not create backupDir.
func Backup(path, backupDir string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("backup of %s: is a directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	name := fmt.Sprintf("%s.%s", filepath.Base(path), time.Now().Format(backupTimeLayout))
	dst := filepath.Join(backupDir, name)
	if err := WriteAtomic(dst, data, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("writing backup %s: %w", dst, err)
	}
	return dst, nil
}

// RotateBackups keeps the newest keep backups named prefix* in backupDir.
// keep <= 0 is a no-op, so a zero value never deletes everything.
func RotateBackups(backupDir, prefix string, keep int) error {
	if keep <= 0 {
		return nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading backups directory %s: %w", backupDir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return nil
	}

	sort.Strings(names) // oldest first
	toRemove := names[:len(names)-keep]
	for _, n := range toRemove {
		if err := os.Remove(filepath.Join(backupDir, n)); err != nil {
			return fmt.Errorf("removing old backup %s: %w", n, err)
		}
	}
	return nil
}
