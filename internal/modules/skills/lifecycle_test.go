package skills

import (
	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"os"
	"testing"
)

func TestSourceDraftFailureDiscardAndFieldNavigation(t *testing.T) {
	m := newTab(New(testPaths(t)))
	m.width, m.height = 40, 13
	m.sourceForm(nil)
	m.input.SetValue("Fixture")
	m, _ = m.updateSourceForm(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.sources.field != 1 || m.input.Value() != "" {
		t.Fatal("field transition lost draft or focus")
	}
	m.input.SetValue("example/repo")
	m, _ = m.updateSourceForm(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.draftDiscard == nil {
		t.Fatal("source discarded without confirmation")
	}
	m, _ = m.update(tea.PasteMsg{Content: "ignored"})
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != skModeSourceForm || m.input.Value() != "example/repo" || m.sources.form.Name != "Fixture" {
		t.Fatal("cancel lost source draft")
	}
	m.sources.savingForm = true
	m.mode = skModeSourceBusy
	m, _ = m.update(sourceSavedMsg{err: os.ErrPermission})
	if m.mode != skModeSourceForm || !m.input.Focused() || m.input.Value() != "example/repo" || m.sources.err == "" {
		t.Fatal("failed save lost source draft")
	}
	m, _ = m.updateSourceForm(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = m.update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.mode != skModeSources || m.draftDiscard != nil || m.sources.err != "" {
		t.Fatal("explicit discard did not close form")
	}
}

func TestPendingSkillCreateCannotRepeatOrLoseInput(t *testing.T) {
	m := newTab(New(testPaths(t)))
	m.mode = skModeNew
	m.input.SetValue("new-skill")
	var cmd tea.Cmd
	m, cmd = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.writing || cmd == nil {
		t.Fatal("create did not capture input")
	}
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyEscape}, tea.PasteMsg{Content: "other"}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}} {
		var next tea.Cmd
		m, next = m.update(msg)
		if next != nil || !m.writing || m.input.Value() != "new-skill" {
			t.Fatal("pending creation escaped or repeated")
		}
	}
	// Inject a write failure, then keep the name for an explicit new attempt.
	m, _ = m.update(createdMsg{err: os.ErrPermission})
	if m.writing || m.mode != skModeNew || !m.input.Focused() {
		t.Fatal("failure did not resume editing")
	}
}

func TestSkillRecoveryKeepsReportAndFailedScansKeepEvidence(t *testing.T) {
	m := newTab(New(testPaths(t)))
	m.width, m.height = 80, 24
	p := &components.OperationReport{Title: "Completed attempt"}
	m.report = p
	m, _ = m.update(openBackupsMsg{})
	m, _ = m.update(listBackupsMsg{backups: []Backup{{SkillDir: "one", Path: "one.tar.gz"}}})
	if m.report != nil || m.suspendedReport != p || m.mode != skModeBackup {
		t.Fatal("recovery lost report")
	}
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.report != p || m.suspendedReport != nil {
		t.Fatal("closing recovery did not resume result")
	}
	m.report = nil
	m, _ = m.update(scannedMsg{skills: []Skill{{Dir: "keep", Name: "keep", InLibrary: true}}})
	m, _ = m.update(scannedMsg{err: os.ErrPermission})
	m, _ = m.update(skToastExpire{seq: m.toastSeq})
	if len(m.skills) != 1 || m.scanErr == "" || m.toast == "" || !m.scanned {
		t.Fatal("failed scan erased or hid stale evidence")
	}
}
