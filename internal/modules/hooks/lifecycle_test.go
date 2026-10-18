package hooks

import (
	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"os"
	"strings"
	"testing"
)

func TestHookDraftSurvivesReviewAndPendingSave(t *testing.T) {
	svc, _ := testService(t)
	m := newTab(svc)
	m.width, m.height = 40, 13
	h := Hook{Name: "draft", Hooks: []agent.Hook{{Event: agent.HookStop, Command: "echo before"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	docs, err := svc.documents(h, 0)
	if err != nil {
		t.Fatal(err)
	}
	r := &hookReader{hook: h, docs: docs, loaded: true}
	m.reader = r
	m.Update(hookEditedMsg{r, docs[0], "echo after", nil})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.confirm != nil || r.draft == nil {
		t.Fatal("review cancellation lost edited command")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if r.discard == nil {
		t.Fatal("draft closed without discard decision")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.reader != r || r.draft == nil {
		t.Fatal("cancel lost reader draft")
	}
	m.Update(hookEditedMsg{r, docs[0], "echo after", os.ErrPermission})
	if r.draft == nil || m.confirm != nil || !strings.Contains(m.View(), "echo after") {
		t.Fatal("editor failure lost available edited text")
	}
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	save := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if !m.writing || save == nil {
		t.Fatal("save did not capture input")
	}
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 's', Text: "s"}, tea.KeyPressMsg{Code: tea.KeyEscape}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}, tea.PasteMsg{Content: "ignored"}} {
		if m.Update(msg) != nil || !m.writing {
			t.Fatal("pending hook save escaped/repeated")
		}
	}
	m.Update(save())
	if m.writing || r.draft != nil {
		t.Fatal("completed save retained executable draft")
	}
	actual, err := svc.Get("draft")
	if err != nil || actual.Hooks[0].Command != "echo after" {
		t.Fatal("reviewed command not saved")
	}
	if m.Update(tea.KeyPressMsg{Code: 's', Text: "s"}) != nil {
		t.Fatal("completed write replayed")
	}
}
