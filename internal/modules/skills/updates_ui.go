package skills

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type reviewReadyMsg struct {
	review *UpdateReview
	err    error
	single bool
	op     *sourceOperation
}
type reviewAppliedMsg struct {
	result core.OperationResult
	op     *sourceOperation
}

func (m Tab) startUpdateReview(skills []Skill, single bool) (Tab, tea.Cmd) {
	spin := m.beginSource("Preparing skill update review…", skModeUpdateBusy, skModeList)
	svc, op := m.svc, m.sourceOp
	return m, tea.Batch(spin, op.command(func(ctx context.Context, notify func(string)) tea.Msg {
		r, err := svc.ReviewUpdates(ctx, skills, notify)
		if r != nil {
			op.own(r.Close)
		}
		return reviewReadyMsg{r, err, single, op}
	}))
}

func (m *Tab) cacheUpdates(checks []UpdateCheck) {
	m.updateStatus = make(map[string]UpdateStatus, len(checks))
	for _, c := range checks {
		m.updateStatus[c.Skill.Dir] = c.Status
	}
}

func reviewState(c UpdateCheck) string {
	if updateIsSkipped(c.Err) {
		return "Skipped: " + c.Err.Error()
	}
	if c.Err != nil {
		return "Check failed: " + c.Err.Error()
	}
	local := "Local content matches the saved install hash."
	if c.Skill.Origin.Hash == "" {
		local = "No saved baseline: local edits cannot be determined."
	} else if c.LocalHash != c.Skill.Origin.Hash {
		local = "Local edits detected against the saved install hash."
	}
	upstream := "Upstream content matches the saved install hash."
	if c.Skill.Origin.Hash == "" {
		upstream = "Upstream changes cannot be separated from local edits without a baseline."
	} else if c.RemoteHash != c.Skill.Origin.Hash {
		upstream = "Upstream content changed since the saved install hash."
	}
	if c.LocalHash == c.RemoteHash {
		upstream += " Local and upstream content are identical."
	}
	return local + "\n" + upstream
}

func (m Tab) reviewViewport() viewport.Model {
	w := max(1, m.width-4)
	footer := m.reviewHints()
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(max(1, m.height-3-lipgloss.Height(footer))))
	if m.review == nil || len(m.review.Checks) == 0 {
		vp.SetContent("No Git skills to review.")
		return vp
	}
	c := m.review.Checks[m.reviewIndex]
	lines := []string{fmt.Sprintf("%d/%d · %s", m.reviewIndex+1, len(m.review.Checks), c.Skill.Dir), reviewState(c)}
	if c.Skill.Origin != nil {
		o := c.Skill.Origin
		ref, path, revision := o.Ref, o.Sub, o.Revision
		if ref == "" {
			ref = "Remote default branch"
		}
		if path == "" {
			path = "Repository root (legacy discovery)"
		}
		if revision == "" {
			revision = "Unknown (legacy origin)"
		}
		lines = append(lines, "Source: "+o.Source, "Ref: "+ref+" · Path: "+path, "Installed commit: "+revision, "Reviewed commit: "+c.Revision)
	}
	if m.reviewSingle {
		lines = append(lines, "Single-skill replacement may overwrite local edits; an archive is saved first.")
	} else {
		lines = append(lines, "Bulk update skips local edits, missing baselines and errors. Every replacement is backed up.")
	}
	lines = append(lines, "Changed files (local → reviewed upstream):")
	if len(c.Files) == 0 {
		if c.Err != nil {
			lines = append(lines, "File comparison unavailable; nothing will be replaced.")
		} else {
			lines = append(lines, "No visible file differences.")
		}
	} else {
		lines = append(lines, c.Files...)
	}
	lines = append(lines, c.Diff)
	vp.SetContent(kit.Wrap(reviewText(strings.Join(lines, "\n")), w, ""))
	vp.SetYOffset(m.reviewOffset)
	return vp
}
func (m Tab) updateReviewView() string {
	footer := m.reviewHints()
	return components.Panel{Title: "Review skill updates", Focused: true, Width: m.width}.Render(m.reviewViewport().View() + "\n" + footer)
}
func (m Tab) updateReview(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		r := m.review
		m.review = nil
		m.mode = skModeList
		return m, func() tea.Msg {
			if r != nil {
				r.Close()
			}
			return nil
		}
	case "left", "h":
		m.reviewIndex = max(0, m.reviewIndex-1)
		m.reviewOffset = 0
	case "right", "l":
		m.reviewIndex = min(max(0, len(m.review.Checks)-1), m.reviewIndex+1)
		m.reviewOffset = 0
	case "enter":
		lines := []string{"Replace the following library copies with the reviewed staged revisions?", "All agent readers use these copies. Save an archive before each replacement."}
		eligible := 0
		for _, c := range m.review.Checks {
			if c.Err != nil || c.LocalHash == c.RemoteHash {
				continue
			}
			if !m.reviewSingle && (c.Status == UpdateStatusLocallyEdited || c.Skill.Origin.Hash == "") {
				continue
			}
			lines = append(lines, "- "+c.Skill.Dir+" @ "+c.Revision)
			eligible++
		}
		if eligible == 0 {
			m.setToast("No eligible replacements; local edits and missing baselines require single-skill review", false)
			return m, nil
		}
		if m.reviewSingle {
			lines = append(lines, "Explicit single-item replacement: any local edits shown in the diff will be overwritten.")
		}
		m.confirm = components.NewConfirm(reviewText(strings.Join(lines, "\n")))
		m.ckind, m.mode = confirmKindReviewedUpdate, skModeConfirm
	default:
		vp := m.reviewViewport()
		vp, _ = vp.Update(msg)
		m.reviewOffset = vp.YOffset()
	}
	return m, nil
}

func (m Tab) applyUpdateReview() (Tab, tea.Cmd) {
	r, force, svc := m.review, m.reviewSingle, m.svc
	m.review = nil
	spin := m.beginSource("Applying reviewed skill updates…", skModeUpdateBusy, skModeList)
	op := m.sourceOp
	op.own(r.Close)
	return m, tea.Batch(spin, op.command(func(ctx context.Context, notify func(string)) tea.Msg {
		defer r.Close()
		return reviewAppliedMsg{svc.ApplyUpdates(ctx, r, force, notify), op}
	}))
}

func (m Tab) reviewHints() string {
	text := "←→ item · ↑↓/pgup/pgdn scroll\nenter review · esc cancel"
	if m.toast != "" {
		text = m.toast + "\n" + text
	}
	return kit.Wrap(text, max(1, m.width-4), "")
}
