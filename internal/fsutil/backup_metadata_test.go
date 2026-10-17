package fsutil

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBackupProvenanceAndRotation(t *testing.T) {
	dir, backups := t.TempDir(), t.TempDir()
	path := filepath.Join(dir, "settings.json")
	content := []byte(`{"token":"private"}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	origin := BackupOrigin{Agent: "claude-code", Operation: "provider.apply"}
	first, err := Backup(path, backups, origin)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(BackupMetadataPath(first))
	if err != nil {
		t.Fatal(err)
	}
	var r BackupRecord
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Version != 1 || r.Destination != path || r.Agent != origin.Agent || r.Operation != origin.Operation || r.SHA256 != fmt.Sprintf("%x", sha256.Sum256(content)) {
		t.Fatalf("bad provenance: %+v", r)
	}
	info, err := os.Stat(BackupMetadataPath(first))
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("metadata permissions")
	}
	if _, err := Backup(path, backups, origin); err != nil {
		t.Fatal(err)
	}
	if err := RotateBackups(backups, "settings.json.", 1); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{first, BackupMetadataPath(first)} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("rotation retained old snapshot or provenance")
		}
	}
}

func TestConditionalWriteRunsExtraGuardBeforeRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	err := WriteAtomicIfUnchanged(path, []byte("current"), true, []byte("restore"), 0o600, func() error {
		called = true
		temps, err := filepath.Glob(path + ".tmp-*")
		if err != nil || len(temps) != 1 {
			t.Fatal("guard did not run after preparing the replacement")
		}
		return ErrChanged
	})
	if !called || !errors.Is(err, ErrChanged) {
		t.Fatalf("guard ignored: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "current" {
		t.Fatal("failed guard changed destination")
	}
	if temps, err := filepath.Glob(path + ".tmp-*"); err != nil || len(temps) != 0 {
		t.Fatal("failed guard left a temporary file")
	}
}

func TestBackupProvenanceFailureIsReported(t *testing.T) {
	dir, backups := t.TempDir(), t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backups, ".config-backup-meta"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(path, backups, BackupOrigin{Agent: "claude-code", Operation: "provider.apply"}); err == nil {
		t.Fatal("metadata failure ignored")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "original" {
		t.Fatal("backup modified the live source")
	}
}
