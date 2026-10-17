package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// OperationReport stays open until explicitly dismissed. Retry captures only
// failed IDs; it never repeats the original bulk selection.
type OperationReport struct {
	Title    string
	Result   core.OperationResult
	Recovery string
	Recover  tea.Cmd
	Retry    func([]string) core.OperationResult
	busy     bool
	confirm  *Confirm
	scroll   int
}

type OperationFinished struct {
	Report *OperationReport
	Result core.OperationResult
}

func (p *OperationReport) Finish(msg OperationFinished) {
	p.Result.MergeRetry(msg.Result)
	p.busy = false
}

func (p *OperationReport) Update(msg tea.Msg, width, height int) (dismiss bool, cmd tea.Cmd) {
	if p.busy {
		return false, nil
	}
	if p.confirm != nil {
		c, answer := p.confirm.Update(msg, width, height)
		p.confirm = &c
		if answer == No {
			p.confirm = nil
		}
		if answer == Yes {
			p.confirm = nil
			ids := p.Result.FailedIDs()
			if len(ids) > 0 && p.Retry != nil {
				p.busy = true
				return false, func() tea.Msg { return OperationFinished{Report: p, Result: p.Retry(ids)} }
			}
		}
		return false, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc", "q", "enter":
			return true, nil
		case "b":
			if p.Recover != nil {
				return true, p.Recover
			}
		case "r":
			if ids := p.Result.FailedIDs(); len(ids) > 0 && p.Retry != nil {
				c := NewConfirm(fmt.Sprintf("Retry only failed targets: %s?\nSuccessful and skipped targets will not be retried. Current saved settings will be used.", strings.Join(ids, ", ")))
				p.confirm = &c
			}
			return false, nil
		}
	}
	vp := p.viewport(width, height)
	vp, _ = vp.Update(msg)
	p.scroll = vp.YOffset()
	return false, nil
}

func (p OperationReport) View(width, height int) string {
	if p.confirm != nil {
		return p.confirm.ViewIn(width, height)
	}
	vp := p.viewport(width, height)
	hint := "↑↓ scroll · esc dismiss"
	if len(p.Result.FailedIDs()) > 0 {
		hint += " · r retry failed"
	}
	if p.Recover != nil {
		hint += " · b recovery"
	}
	if p.busy {
		hint = "Retrying failed targets…"
	}
	hint = lipgloss.NewStyle().Width(max(1, width-4)).Render(hint)
	return Panel{Title: p.Title, Focused: true, Width: max(1, width)}.Render(vp.View() + "\n" + hint)
}

func (p OperationReport) viewport(width, height int) viewport.Model {
	w := max(1, width-4)
	var lines []string
	for _, t := range p.Result.Targets {
		lines = append(lines, fmt.Sprintf("%s: %s", t.ID, t.Outcome))
		if t.Detail != "" {
			lines = append(lines, "  "+t.Detail)
		}
		if t.State != "" {
			lines = append(lines, "  Current: "+t.State)
		}
		lines = append(lines, "")
	}
	if p.Result.Err != nil {
		lines = append(lines, p.Result.Err.Error(), "")
	}
	lines = append(lines, "Changes are per target. A failure may include partial changes.", "Recovery: "+p.Recovery)
	footer := lipgloss.Height(lipgloss.NewStyle().Width(w).Render("↑↓ scroll · esc dismiss · r retry failed · b recovery"))
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(max(1, height-footer-3)))
	vp.SetContent(lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n")))
	vp.SetYOffset(p.scroll)
	return vp
}
