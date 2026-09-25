package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// deleteSessionFile backs the session file up into backupsDir, then removes it.
// Copies instead of renaming so backupsDir can be on another filesystem.
func deleteSessionFile(path, backupsDir string) error {
	if path == "" {
		return fmt.Errorf("session has no file path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("checking session: %w", err)
	}
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		return fmt.Errorf("creating backup dir: %w", err)
	}
	ts := time.Now().Format("20060102T150405.000000000")
	dst := filepath.Join(backupsDir, fmt.Sprintf("%s.%s", filepath.Base(path), ts))
	if err := copyFilePerm(path, dst, info.Mode().Perm()); err != nil {
		return fmt.Errorf("backing up session: %w", err)
	}
	if err := os.Remove(path); err != nil {
		_ = os.Remove(dst) // rollback backup on remove failure
		return fmt.Errorf("removing session: %w", err)
	}
	return nil
}

func copyFilePerm(src, dst string, perm os.FileMode) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		_ = d.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		_ = os.Remove(dst)
		return err
	}
	return d.Close()
}
