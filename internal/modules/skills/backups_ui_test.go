package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRestoreLastRemovedSkillFromEmptyTab(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "deleted-skill", "original")
	for _, args := range [][]string{{"enable", "deleted-skill"}, {"remove", "deleted-skill"}} {
		if _, stderr, code := run(t, args, svc, agents); code != 0 {
			t.Fatal(stderr)
		}
	}
	m := newTab(New(svc.Paths())) // reopen with an empty library
	m.agents = agents
	m, _ = m.update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m, cmd := m.updateList(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if cmd == nil {
		t.Fatal("backups are unreachable without a selected skill")
	}
	m, _ = m.update(cmd())
	if m.mode != skModeBackup || m.backupPicker.selected().SkillDir != "deleted-skill" {
		t.Fatalf("deleted skill missing from backups: %+v", m.backupPicker)
	}
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(m.View(), "enablements are unchanged") {
		t.Fatal("restore confirmation does not explain activation state")
	}
	m, cmd = m.updateConfirm(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("restore confirmation produced no action")
	}
	m, cmd = m.update(cmd())
	if m.toastErr {
		t.Fatal(m.toast)
	}
	m, _ = m.update(cmd()) // rescan restored files
	if len(m.skills) != 1 || !m.skills[0].InLibrary {
		t.Fatalf("restored skill missing from the library: %+v", m.skills)
	}
	if _, err := os.Lstat(filepath.Join(agents[0].ManagedDir, "deleted-skill")); !os.IsNotExist(err) {
		t.Fatalf("restore recreated an activation: %v", err)
	}
}

func TestBackupPickerSmallTerminal(t *testing.T) {
	backups := make([]Backup, 20)
	for i := range backups {
		backups[i] = Backup{SkillDir: fmt.Sprintf("skill-%02d", i), Path: fmt.Sprintf("/backups/skill-%02d.tar.gz", i), Time: time.Date(2026, 9, i+1, 12, 0, 0, 0, time.UTC)}
	}
	picker := newBackupPicker(backups, "skill")
	for _, width := range []int{30, 36, 76} {
		for i := range backups {
			picker.cursor = i
			view := picker.view(width, 11)
			plain := ansi.Strip(view)
			if lipgloss.Height(view) > 11 || lipgloss.Width(view) > width || !strings.Contains(plain, backups[i].SkillDir) || !strings.Contains(plain, backups[i].Time.Format("2006-01-02")) || !strings.Contains(plain, "esc") {
				t.Fatalf("backup %d unreachable at %d columns:\n%s", i, width, plain)
			}
		}
	}
}

func TestBackupInventoryAndSearch(t *testing.T) {
	m := matrixTab(t, 1, 80, 20)
	// B and the palette request an unfiltered inventory despite a selected row.
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 'B', Text: "B"}, m.Commands()[0].Msg} {
		_, cmd := m.update(msg)
		if cmd == nil {
			t.Fatal("all-backups entry point returned no command")
		}
		if got := cmd().(listBackupsMsg); got.skillDir != "" {
			t.Fatalf("inventory is still restricted to %q", got.skillDir)
		}
	}
	backups := []Backup{
		{SkillDir: "deleted-pdf", Path: "/backups/pdf-new.tar.gz", Time: time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)},
		{SkillDir: "other", Path: "/backups/other.tar.gz", Time: time.Date(2026, 10, 17, 11, 0, 0, 0, time.Local)},
		{SkillDir: "deleted-pdf", Path: "/backups/pdf-old.tar.gz", Time: time.Date(2026, 10, 16, 12, 0, 0, 0, time.Local)},
	}
	m, _ = m.update(listBackupsMsg{backups: backups})
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.update(tea.PasteMsg{Content: "PDF 2026-10-17"})
	if !strings.Contains(m.View(), "apply filter") {
		t.Fatal("filter footer still offers restore while editing")
	}
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != skModeBackup || len(m.backupPicker.matches) != 1 || m.backupPicker.selected().Path != backups[0].Path {
		t.Fatalf("name/date search selected the wrong archive: %+v", m.backupPicker)
	}
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.updateConfirm(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != skModeBackup || len(m.backupPicker.matches) != 1 {
		t.Fatal("cancelling restore lost the filtered inventory")
	}
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.update(tea.PasteMsg{Content: " no-match"})
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, cmd := m.updateBackupPicker(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || m.mode != skModeBackup || !strings.Contains(m.View(), "No backups match") {
		t.Fatal("empty search attempted a restore")
	}
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.updateBackupPicker(tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.backupPicker.matches) != len(backups) || m.backupPicker.filtering {
		t.Fatal("escape did not clear the filter")
	}
}
