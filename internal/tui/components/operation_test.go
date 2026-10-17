package components

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestOperationReportRetryConfirmationAndBounds(t *testing.T) {
	var called []string
	p := &OperationReport{Title: "Bulk operation", Result: core.OperationResult{Targets: []core.TargetResult{
		{ID: "good", Outcome: core.Succeeded, State: "configured"},
		{ID: "bad", Outcome: core.Failed, Detail: strings.Repeat("failure ", 80)},
		{ID: "offline", Outcome: core.Skipped, Detail: "not installed"},
	}}, Recovery: "Recovery tab", Retry: func(ids []string) core.OperationResult {
		called = ids
		return core.OperationResult{Targets: []core.TargetResult{{ID: "bad", Outcome: core.Succeeded, State: "configured"}}}
	}}
	for _, w := range []int{40, 100} {
		view := p.View(w, 18)
		if lipgloss.Width(view) > w || lipgloss.Height(view) > 18 {
			t.Fatalf("report overflows %dx18: %dx%d", w, lipgloss.Width(view), lipgloss.Height(view))
		}
	}
	p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, 80, 24)
	if p.confirm == nil || !strings.Contains(p.confirm.Question, "only failed targets: bad") {
		t.Fatal("retry does not identify failures")
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, 80, 24)
	if called != nil || p.confirm != nil {
		t.Fatal("cancelled retry ran")
	}
	p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, 80, 24)
	dismiss, cmd := p.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}, 80, 24)
	if dismiss || cmd == nil {
		t.Fatal("missing retry")
	}
	p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, 80, 24) // no overlapping retry
	p.Finish(cmd().(OperationFinished))
	if !reflect.DeepEqual(called, []string{"bad"}) || len(p.Result.FailedIDs()) != 0 || len(p.Result.Targets) != 3 {
		t.Fatalf("retry=%v result=%+v", called, p.Result)
	}
	if dismiss, cmd = p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, 80, 24); dismiss || cmd != nil || p.confirm != nil {
		t.Fatal("empty failure set repeated bulk operation")
	}
	if dismiss, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, 80, 24); !dismiss {
		t.Fatal("report did not dismiss")
	}
}
