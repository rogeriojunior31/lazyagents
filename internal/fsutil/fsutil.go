// Package fsutil owns every disk write: WriteAtomic (tmp + rename), Backup and
// RotateBackups.
package fsutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	return writeAtomic(path, data, perm, nil)
}

// ErrChanged is a file that changed on disk after it was read: another
// program wrote it in between, and overwriting would lose that write.
var ErrChanged = errors.New("changed on disk since it was read")

// WriteAtomicIfUnchanged is WriteAtomic only while path still holds before
// (existed=false: path did not exist). The check runs right before the
// rename, so another lazyagents or the agent itself writing the same file in
// the meantime gets ErrChanged instead of being overwritten; the gap left is
// that instant. Extra guards verify related state at that same final check.
func WriteAtomicIfUnchanged(path string, before []byte, existed bool, data []byte, perm os.FileMode, guards ...func() error) error {
	return writeAtomic(path, data, perm, func() error {
		now, err := os.ReadFile(path)
		switch {
		case os.IsNotExist(err) && !existed:
		case err != nil && !os.IsNotExist(err):
			return fmt.Errorf("reading %s: %w", path, err)
		case os.IsNotExist(err) != !existed, !bytes.Equal(now, before):
			return fmt.Errorf("%s %w", path, ErrChanged)
		}
		for _, guard := range guards {
			if err := guard(); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveIfUnchanged removes path only while it still holds before; another
// program's write since the read gets ErrChanged instead of being deleted.
func RemoveIfUnchanged(path string, before []byte) error {
	now, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if !bytes.Equal(now, before) {
		return fmt.Errorf("%s %w", path, ErrChanged)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

func writeAtomic(path string, data []byte, perm os.FileMode, check func() error) error {
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
	if check != nil {
		if err := check(); err != nil {
			return err
		}
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// Backup copies path into backupDir as "<basename>.<timestamp>", keeping the
// original mode (it may hold secrets), and returns the backup path. A missing
// path is a no-op returning ("", nil) and does not create backupDir.
func Backup(path, backupDir string, origin ...BackupOrigin) (string, error) {
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

	now := time.Now()
	name := fmt.Sprintf("%s.%s", filepath.Base(path), now.Format(backupTimeLayout))
	dst := filepath.Join(backupDir, name)
	if err := WriteAtomic(dst, data, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("writing backup %s: %w", dst, err)
	}
	if len(origin) > 0 {
		target, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		record := BackupRecord{Version: 1, BackupOrigin: origin[0], Destination: target,
			Time: now, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Mode: uint32(info.Mode().Perm())}
		metadata, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return "", err
		}
		if err := WriteAtomic(BackupMetadataPath(dst), append(metadata, '\n'), 0o600); err != nil {
			return "", fmt.Errorf("recording backup origin: %w", err)
		}
	}
	return dst, nil
}

// BackupOrigin is supplied by an adapter, never inferred from a basename.
type BackupOrigin struct {
	Agent     string `json:"agent"`
	Operation string `json:"operation"`
}

// BackupRecord contains provenance and integrity data, never config values.
type BackupRecord struct {
	Version int `json:"version"`
	BackupOrigin
	Destination string    `json:"destination"`
	Time        time.Time `json:"time"`
	SHA256      string    `json:"sha256"`
	Mode        uint32    `json:"mode"`
}

func BackupMetadataPath(backup string) string {
	return filepath.Join(filepath.Dir(backup), ".config-backup-meta", filepath.Base(backup)+".json")
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
		if err := os.Remove(BackupMetadataPath(filepath.Join(backupDir, n))); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing old backup metadata: %w", err)
		}
	}
	return nil
}
