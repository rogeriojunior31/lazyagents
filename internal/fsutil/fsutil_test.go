package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteAtomic(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(dir string) string // returns the target path
		data    []byte
		perm    os.FileMode
		preData []byte // existing content to overwrite
	}{
		{
			name:  "new file",
			setup: func(dir string) string { return filepath.Join(dir, "new.txt") },
			data:  []byte("new content"),
			perm:  0o644,
		},
		{
			name:    "overwrite",
			setup:   func(dir string) string { return filepath.Join(dir, "exists.txt") },
			data:    []byte("final content"),
			perm:    0o644,
			preData: []byte("old content much longer than the new one"),
		},
		{
			name:  "perm 0600",
			setup: func(dir string) string { return filepath.Join(dir, "secret.json") },
			data:  []byte(`{"apiKey":"sk-xxx"}`),
			perm:  0o600,
		},
		{
			name:  "creates missing parent dir",
			setup: func(dir string) string { return filepath.Join(dir, "sub", "dir", "f.txt") },
			data:  []byte("nested"),
			perm:  0o600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tt.setup(dir)
			if tt.preData != nil {
				if err := os.WriteFile(path, tt.preData, 0o644); err != nil {
					t.Fatalf("setup existing file: %v", err)
				}
			}

			if err := WriteAtomic(path, tt.data, tt.perm); err != nil {
				t.Fatalf("WriteAtomic: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading result: %v", err)
			}
			if string(got) != string(tt.data) {
				t.Errorf("content = %q, want %q", got, tt.data)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != tt.perm { // Windows has no Unix modes
				t.Errorf("perm = %o, want %o", info.Mode().Perm(), tt.perm)
			}

			// No temp file may be left behind.
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatalf("readdir: %v", err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), ".tmp-") {
					t.Errorf("temp file not removed: %s", e.Name())
				}
			}
		})
	}
}

func TestWriteAtomicError(t *testing.T) {
	t.Run("fails when the parent dir cannot be created", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "a-file")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatalf("setup: %v", err)
		}
		// file is a regular file, so file/sub cannot exist.
		target := filepath.Join(file, "sub", "f.txt")
		if err := WriteAtomic(target, []byte("y"), 0o600); err == nil {
			t.Error("WriteAtomic should fail when the parent dir cannot be created")
		}
	})
}

func TestBackup(t *testing.T) {
	t.Run("existing file is copied with timestamp and mode", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "config.json")
		content := []byte(`{"apiKey":"sk-secret"}`)
		if err := os.WriteFile(src, content, 0o600); err != nil {
			t.Fatalf("setup: %v", err)
		}
		backupDir := filepath.Join(dir, "backups")

		got, err := Backup(src, backupDir)
		if err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if got == "" {
			t.Fatal("Backup returned an empty path for an existing file")
		}
		if filepath.Dir(got) != backupDir {
			t.Errorf("backup created in %s, want inside %s", filepath.Dir(got), backupDir)
		}
		if !strings.HasPrefix(filepath.Base(got), "config.json.") {
			t.Errorf("backup name = %q, want prefix %q", filepath.Base(got), "config.json.")
		}

		bdata, err := os.ReadFile(got)
		if err != nil {
			t.Fatalf("reading backup: %v", err)
		}
		if string(bdata) != string(content) {
			t.Errorf("backup content = %q, want %q", bdata, content)
		}

		info, err := os.Stat(got)
		if err != nil {
			t.Fatalf("stat backup: %v", err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Errorf("backup perm = %o, want 0600", info.Mode().Perm())
		}
	})

	t.Run("missing file is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "missing.json")
		backupDir := filepath.Join(dir, "backups")

		got, err := Backup(src, backupDir)
		if err != nil {
			t.Fatalf("Backup of a missing file should be a no-op, got error: %v", err)
		}
		if got != "" {
			t.Errorf("Backup of a missing file returned path %q, want empty", got)
		}
		if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
			t.Errorf("backupDir should not be created for a missing file")
		}
	})

	t.Run("backup of a directory fails", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "adir")
		if err := os.Mkdir(src, 0o700); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if _, err := Backup(src, filepath.Join(dir, "backups")); err == nil {
			t.Error("Backup of a directory should fail")
		}
	})
}

func TestRotateBackups(t *testing.T) {
	const prefix = "config.json."

	// Oldest to newest; lexical order is chronological.
	mkNames := func(n int) []string {
		stamps := []string{
			"20240101T000001.000000000",
			"20240101T000002.000000000",
			"20240101T000003.000000000",
			"20240101T000004.000000000",
			"20240101T000005.000000000",
		}
		names := make([]string, n)
		for i := 0; i < n; i++ {
			names[i] = prefix + stamps[i]
		}
		return names
	}

	tests := []struct {
		name      string
		nBackups  int
		keep      int
		extra     []string // files outside the prefix; must survive
		wantKept  int      // prefixed backups left
		keepNewer bool     // the kept ones must be the newest
	}{
		{name: "keeps the N newest", nBackups: 5, keep: 3, wantKept: 3, keepNewer: true},
		{name: "keep above total keeps all", nBackups: 2, keep: 5, wantKept: 2},
		{name: "keep zero is a no-op", nBackups: 3, keep: 0, wantKept: 3},
		{name: "negative keep is a no-op", nBackups: 3, keep: -1, wantKept: 3},
		{name: "leaves other prefixes alone", nBackups: 4, keep: 1, wantKept: 1,
			extra: []string{"settings.json.20240101T000009.000000000", "other.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			names := mkNames(tt.nBackups)
			for _, n := range names {
				if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			for _, n := range tt.extra {
				if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
					t.Fatalf("setup extra: %v", err)
				}
			}

			if err := RotateBackups(dir, prefix, tt.keep); err != nil {
				t.Fatalf("RotateBackups: %v", err)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("readdir: %v", err)
			}
			var kept []string
			survivors := map[string]bool{}
			for _, e := range entries {
				survivors[e.Name()] = true
				if strings.HasPrefix(e.Name(), prefix) {
					kept = append(kept, e.Name())
				}
			}
			if len(kept) != tt.wantKept {
				t.Errorf("%d prefixed backups left, want %d (%v)", len(kept), tt.wantKept, kept)
			}
			for _, n := range tt.extra {
				if !survivors[n] {
					t.Errorf("file %q was wrongly removed", n)
				}
			}
			if tt.keepNewer {
				want := names[tt.nBackups-tt.keep:] // newest
				for _, w := range want {
					if !survivors[w] {
						t.Errorf("newest backup %q should have been kept", w)
					}
				}
			}
		})
	}

	t.Run("missing backups dir is a no-op", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")
		if err := RotateBackups(dir, prefix, 3); err != nil {
			t.Errorf("RotateBackups on a missing dir should be a no-op, got: %v", err)
		}
	})
}
