package skills

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestInstallRejectsEscapingName(t *testing.T) {
	p := testPaths(t)
	src := t.TempDir()
	writeSkill(t, src, "source", validMD("../../escaped", "bad"))
	svc := New(p)
	found, origin, _, err := svc.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := svc.Install(found, origin)
	if err == nil || len(installed) != 0 {
		t.Fatalf("installed=%v err=%v", installed, err)
	}
	if _, err := os.Stat(filepath.Join(p.DataDir, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("escaped: %v", err)
	}
}

func TestRestoreCorruptBackupKeepsExistingSkill(t *testing.T) {
	p := testPaths(t)
	writeSkill(t, p.LibraryDir(), "keep", validMD("keep", "original"))
	file := filepath.Join(p.LibraryDir(), "keep", "SKILL.md")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := os.WriteFile(bad, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := New(p).Restore(Backup{SkillDir: "keep", Path: bad}); err == nil {
		t.Fatal("accepted corrupt archive")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(before) != string(after) {
		t.Fatalf("original changed: %v", err)
	}
}

func TestBackupsSortedAcrossSkills(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.BackupsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.20260922T120000.tar.gz", "z.20260921T120000.tar.gz"} {
		if err := os.WriteFile(filepath.Join(p.BackupsDir(), name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := New(p).ListBackups()
	if err != nil || len(got) != 2 || !got[0].Time.After(got[1].Time) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestMigrationRejectsConflictAndInvalidConfig(t *testing.T) {
	for _, scenario := range []string{"conflict", "config", "nested"} {
		t.Run(scenario, func(t *testing.T) {
			p := testPaths(t)
			writeSkill(t, p.LibraryDir(), "keep", validMD("keep", "original"))
			dst := filepath.Join(t.TempDir(), "target")
			if err := (core.Config{Theme: "garoa"}).Save(p.ConfigPath()); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "conflict":
				writeSkill(t, dst, "keep", validMD("keep", "foreign"))
			case "config":
				if err := os.WriteFile(p.ConfigPath(), []byte("[bad"), 0600); err != nil {
					t.Fatal(err)
				}
			case "nested":
				dst = filepath.Join(p.LibraryDir(), "nested")
			}
			before, _ := os.ReadFile(p.ConfigPath())
			if err := New(p).MigrateLibrary(dst, nil); err == nil {
				t.Fatal("expected refusal")
			}
			after, _ := os.ReadFile(p.ConfigPath())
			if string(before) != string(after) {
				t.Fatal("config changed")
			}
			if _, err := os.Stat(filepath.Join(p.LibraryDir(), "keep", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRestoreRejectsUnsafeName(t *testing.T) {
	if err := New(testPaths(t)).Restore(Backup{SkillDir: "../escape", Time: time.Now()}); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestNullProfiles(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.DataDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfilesPath(), []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := New(p).SaveProfile("empty", ProfileSpec{}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMissingSourcePreservesContents(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "skill")
	writeSkill(t, filepath.Dir(dst), "skill", validMD("skill", "original"))
	if err := replaceDir(filepath.Join(t.TempDir(), "missing"), dst); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatal("lost original", err)
	}
}

func TestZipRejectsOversizedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("large.txt")
	if err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte("x"), 1<<20)
	for i := 0; i < 65; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if tmp, err := extractZip(path); err == nil {
		os.RemoveAll(tmp)
		t.Fatal("silently truncated oversized entry")
	}
}
