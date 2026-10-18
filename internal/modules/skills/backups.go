package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// backupPickerState browses one skill's backups or the full inventory.
type backupPickerState struct {
	backups   []Backup
	matches   []Backup
	skillDir  string
	cursor    int
	filter    textinput.Model
	filtering bool
}

func newBackupPicker(backups []Backup, skillDir string) backupPickerState {
	input := components.NewInput()
	input.Prompt = "/ "
	input.Placeholder = "Skill name or YYYY-MM-DD"
	return backupPickerState{backups: backups, matches: backups, skillDir: skillDir, filter: input}
}

func (p *backupPickerState) update(msg tea.Msg) tea.Cmd {
	if p.filtering {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "enter":
				p.filtering = false
				p.filter.Blur()
				return nil
			case "esc":
				p.filtering = false
				p.filter.Blur()
				p.filter.SetValue("")
				p.matches, p.cursor = p.backups, 0
				return nil
			}
		}
		before := p.filter.Value()
		var cmd tea.Cmd
		p.filter, cmd = p.filter.Update(msg)
		if p.filter.Value() != before {
			p.matches, p.cursor = filterBackups(p.backups, p.filter.Value()), 0
		}
		return cmd
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "/":
		p.filtering = true
		return p.filter.Focus()
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.matches)-1 {
			p.cursor++
		}
	}
	return nil
}

func (p backupPickerState) selected() Backup {
	if p.cursor >= 0 && p.cursor < len(p.matches) {
		return p.matches[p.cursor]
	}
	return Backup{}
}

func (p backupPickerState) window(height int) (int, int) {
	return kit.Window(p.cursor, len(p.matches), max(1, (height-5)/2))
}

func (p backupPickerState) view(maxW, maxH int) string {
	var b strings.Builder
	b.WriteString(components.InputView(p.filter, maxW-4) + "\n")
	start, end := p.window(maxH)
	for i := start; i < end; i++ {
		bk := p.matches[i]
		ts := bk.Time.Format("2006-01-02 15:04:05")
		line := bk.SkillDir
		if i == p.cursor {
			line = kit.StOn.Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n  " + kit.StHint.Render(ts) + "\n")
	}
	if len(p.matches) == 0 {
		b.WriteString(kit.StHint.Render("No backups match this filter") + "\n")
	}
	hints := kit.Hints(maxW-4, [2]string{"enter", "restore"}, [2]string{"/", "filter"}, [2]string{"esc", "back"})
	if p.filtering {
		hints = kit.Hints(maxW-4, [2]string{"enter", "apply filter"}, [2]string{"esc", "clear filter"})
	}
	b.WriteString("\n" + hints)
	title := "All skill backups"
	if p.skillDir != "" {
		title = fmt.Sprintf("Backups of %q", p.skillDir)
	}
	title += fmt.Sprintf(" (%d/%d)", min(p.cursor+1, len(p.matches)), len(p.matches))
	return components.Panel{Title: title, Focused: true, Width: maxW}.Render(strings.TrimRight(b.String(), "\n"))
}

func (m *Tab) updateBackupPicker(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	if m.backupPicker.filtering {
		cmd := m.backupPicker.update(msg)
		return *m, cmd
	}
	switch msg.String() {
	case "esc", "q":
		m.mode = skModeList
		m.report, m.suspendedReport = m.suspendedReport, nil
	case "enter":
		sel := m.backupPicker.selected()
		if sel.Path == "" {
			return *m, nil
		}
		m.pendingRestore = sel
		m.ckind = confirmKindRestore
		m.confirm = components.NewConfirm(fmt.Sprintf("Restore backup of %q (%s)?\nReplaces library files, backing up the current folder first.\nAgent enablements are unchanged.", sel.SkillDir, sel.Time.Format("2006-01-02 15:04:05")))
		m.mode = skModeConfirm
	default:
		cmd := m.backupPicker.update(msg)
		return *m, cmd
	}
	return *m, nil
}

func (m *Tab) listBackupsCmd(skillDir string) tea.Cmd {
	m.loadingView = "Skill backups"
	svc := m.svc
	return func() tea.Msg {
		all, err := svc.ListBackups()
		if err != nil {
			return listBackupsMsg{skillDir: skillDir, err: err}
		}
		var filtered []Backup
		for _, b := range all {
			if skillDir == "" || b.SkillDir == skillDir {
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
