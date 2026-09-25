package skills

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// backupPickerState gerencia a lista de backups disponíveis para uma
type backupPickerState struct {
	backups  []Backup
	skillDir string
	cursor   int
}

func newBackupPicker(backups []Backup, skillDir string) backupPickerState {
	return backupPickerState{backups: backups, skillDir: skillDir}
}

func (p *backupPickerState) update(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.backups)-1 {
			p.cursor++
		}
	}
}

func (p backupPickerState) selected() Backup {
	if p.cursor < len(p.backups) {
		return p.backups[p.cursor]
	}
	return Backup{}
}

func (p backupPickerState) view(maxW, maxH int) string {
	var b strings.Builder
	start, end := kit.Window(p.cursor, len(p.backups), maxH-5)
	for i := start; i < end; i++ {
		bk := p.backups[i]
		ts := bk.Time.Format("2006-01-02 15:04:05")
		line := fmt.Sprintf("%s  %s", ts, kit.StHint.Render(kit.Truncate(bk.Path, maxW-34)))
		if i == p.cursor {
			line = kit.StOn.Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" +
		components.Keycap("enter") + kit.StHint.Render(" restore  ") +
		components.Keycap("esc") + kit.StHint.Render(" back"))
	title := fmt.Sprintf("Backups of %q (%d/%d)", p.skillDir, p.cursor+1, len(p.backups))
	return components.Panel{Title: title, Focused: true, Width: maxW}.Render(strings.TrimRight(b.String(), "\n"))
}

func (m *Tab) updateBackupPicker(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = skModeList
	case "enter":
		sel := m.backupPicker.selected()
		if sel.Path == "" {
			return *m, nil
		}
		m.pendingRestore = sel
		m.ckind = confirmKindRestore
		m.confirm = components.NewConfirm(fmt.Sprintf("Restore backup of %q (%s)?", sel.SkillDir, sel.Time.Format("2006-01-02 15:04")))
		m.mode = skModeConfirm
	default:
		m.backupPicker.update(msg)
	}
	return *m, nil
}

func (m Tab) listBackupsCmd(skillDir string) tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		all, err := svc.ListBackups()
		if err != nil {
			return listBackupsMsg{skillDir: skillDir, err: err}
		}
		var filtered []Backup
		for _, b := range all {
			if b.SkillDir == skillDir {
				filtered = append(filtered, b)
			}
		}
		return listBackupsMsg{backups: filtered, skillDir: skillDir}
	}
}

func (m Tab) restoreBackupCmd() tea.Cmd {
	svc, b := m.svc, m.pendingRestore
	return func() tea.Msg {
		err := svc.Restore(b)
		return restoreBackupMsg{skillDir: b.SkillDir, err: err}
	}
}
