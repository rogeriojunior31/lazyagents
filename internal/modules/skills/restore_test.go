package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListBackups_Empty(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	backups, err := svc.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Errorf("want 0 backups, got %d", len(backups))
	}
}

func TestListBackups_And_Restore_RoundTrip(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-restore", validMD("sk-restore", "original"))

	// Remove backs up and deletes
	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}
	var sk Skill
	for _, s := range skills {
		if s.Dir == "sk-restore" {
			sk = s
		}
	}
	if sk.Dir == "" {
		t.Fatal("skill not found by the scan")
	}
	if err := svc.Remove(sk, nil); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk-restore")); !os.IsNotExist(err) {
		t.Error("skill still exists after Remove")
	}

	backups, err := svc.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 {
		t.Fatal("no backup after Remove")
	}
	bk := backups[0]
	if bk.SkillDir != "sk-restore" {
		t.Errorf("backup.SkillDir = %q, want sk-restore", bk.SkillDir)
	}

	if err := svc.Restore(bk); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(p.LibraryDir(), "sk-restore", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading SKILL.md after Restore: %v", err)
	}
	if !strings.Contains(string(data), "original") {
		t.Errorf("restored content lacks 'original': %s", data)
	}
}

func TestRestore_SafetyBackupOnExisting(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-safety", validMD("sk-safety", "v1"))

	// remove to create the v1 backup
	skills, _ := svc.Scan(nil)
	var sk Skill
	for _, s := range skills {
		if s.Dir == "sk-safety" {
			sk = s
		}
	}
	if err := svc.Remove(sk, nil); err != nil {
		t.Fatal(err)
	}

	writeSkill(t, p.LibraryDir(), "sk-safety", validMD("sk-safety", "v2"))

	backups, _ := svc.ListBackups()
	if len(backups) == 0 {
		t.Fatal("no v1 backup")
	}

	// restoring v1 over v2 must back up v2 first
	countBefore := len(backups)
	if err := svc.Restore(backups[0]); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	backupsAfter, _ := svc.ListBackups()
	if len(backupsAfter) <= countBefore {
		t.Error("no safety backup of v2 before restoring")
	}

	data, _ := os.ReadFile(filepath.Join(p.LibraryDir(), "sk-safety", "SKILL.md"))
	if !strings.Contains(string(data), "v1") {
		t.Errorf("want v1 after Restore, got: %s", data)
	}
}

func TestRestore_CorruptedBackup(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	backupsDir := p.BackupsDir()
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Format("20060102T150405")
	badPath := filepath.Join(backupsDir, "sk-bad."+ts+".tar.gz")
	if err := os.WriteFile(badPath, []byte("not a tarball"), 0o600); err != nil {
		t.Fatal(err)
	}

	bk := Backup{SkillDir: "sk-bad", Path: badPath}
	err := svc.Restore(bk)
	if err == nil {
		t.Error("Restore of a corrupt backup should fail")
	}

	if _, err2 := os.Stat(filepath.Join(p.LibraryDir(), "sk-bad")); err2 == nil {
		// acceptable: what matters is that Restore failed
	}
}

func TestRotation_Max20(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	writeSkill(t, p.LibraryDir(), "sk-rot", validMD("sk-rot", "v1"))

	skills, _ := svc.Scan(nil)
	var sk Skill
	for _, s := range skills {
		if s.Dir == "sk-rot" {
			sk = s
		}
	}

	// 22 backups (remove + re-add each time)
	for i := 0; i < 22; i++ {
		writeSkill(t, p.LibraryDir(), "sk-rot", validMD("sk-rot", "v"))
		skills2, _ := svc.Scan(nil)
		for _, s := range skills2 {
			if s.Dir == "sk-rot" {
				sk = s
			}
		}
		_ = svc.Remove(sk, nil)
	}

	backups, err := svc.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, b := range backups {
		if b.SkillDir == "sk-rot" {
			count++
		}
	}
	if count > 20 {
		t.Errorf("rotation should keep at most 20 backups, has %d", count)
	}
}
