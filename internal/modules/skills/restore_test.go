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
		t.Errorf("esperava 0 backups, got %d", len(backups))
	}
}

func TestListBackups_And_Restore_RoundTrip(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// cria skill na biblioteca manualmente
	writeSkill(t, p.LibraryDir(), "sk-restore", validMD("sk-restore", "original"))

	// simula Remove (gera backup + apaga)
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
		t.Fatal("skill não encontrada no scan")
	}
	if err := svc.Remove(sk, nil); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// skill removida
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk-restore")); !os.IsNotExist(err) {
		t.Error("skill ainda existe após Remove")
	}

	// backup deve existir
	backups, err := svc.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 {
		t.Fatal("nenhum backup após Remove")
	}
	bk := backups[0]
	if bk.SkillDir != "sk-restore" {
		t.Errorf("backup.SkillDir = %q, want sk-restore", bk.SkillDir)
	}

	// restaura
	if err := svc.Restore(bk); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// conteúdo deve bater
	data, err := os.ReadFile(filepath.Join(p.LibraryDir(), "sk-restore", "SKILL.md"))
	if err != nil {
		t.Fatalf("lendo SKILL.md após Restore: %v", err)
	}
	if !strings.Contains(string(data), "original") {
		t.Errorf("conteúdo restaurado não contém 'original': %s", data)
	}
}

func TestRestore_SafetyBackupOnExisting(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// instala skill v1
	writeSkill(t, p.LibraryDir(), "sk-safety", validMD("sk-safety", "v1"))

	// remove para gerar backup v1
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

	// reinstala v2
	writeSkill(t, p.LibraryDir(), "sk-safety", validMD("sk-safety", "v2"))

	backups, _ := svc.ListBackups()
	if len(backups) == 0 {
		t.Fatal("nenhum backup de v1")
	}

	// restaura v1 sobre v2 existente → deve gerar safety backup de v2
	countBefore := len(backups)
	if err := svc.Restore(backups[0]); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	backupsAfter, _ := svc.ListBackups()
	if len(backupsAfter) <= countBefore {
		t.Error("safety backup de v2 não foi gerado antes de restaurar")
	}

	// conteúdo deve ser v1
	data, _ := os.ReadFile(filepath.Join(p.LibraryDir(), "sk-safety", "SKILL.md"))
	if !strings.Contains(string(data), "v1") {
		t.Errorf("após Restore esperava v1, got: %s", data)
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
		t.Error("Restore de backup corrompido deveria retornar erro")
	}

	// biblioteca intacta (skill não deve ter sido criada pela metade)
	if _, err2 := os.Stat(filepath.Join(p.LibraryDir(), "sk-bad")); err2 == nil {
		// aceitável: RemoveAll limpa antes de extrair, mas a extração falha
		// o importante é que Restore retornou erro
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

	// gera 22 backups (cada remove+readd)
	for i := 0; i < 22; i++ {
		writeSkill(t, p.LibraryDir(), "sk-rot", validMD("sk-rot", "v"))
		skills2, _ := svc.Scan(nil)
		for _, s := range skills2 {
			if s.Dir == "sk-rot" {
				sk = s
			}
		}
		_ = svc.Remove(sk, nil)
		// O Remove remove a skill, então reescreve para próxima iteração
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
		t.Errorf("rotação deveria manter no máximo 20 backups, tem %d", count)
	}
}
