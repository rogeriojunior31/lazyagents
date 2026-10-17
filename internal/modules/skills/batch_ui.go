package skills

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type batchDraft struct {
	plan           skillBatch
	row            int
	visible, total int
	err            string
}

type skillBatchMsg struct {
	plan   skillBatch
	result core.OperationResult
}

func (m *Tab) mark(dir string) {
	if m.marked == nil {
		m.marked = map[string]bool{}
	}
	if m.marked[dir] {
		delete(m.marked, dir)
	} else {
		m.marked[dir] = true
	}
}

// Hidden or removed rows cannot remain in an actionable selection.
func (m *Tab) reconcileMarks() {
	visible := map[string]bool{}
	for _, it := range m.list.VisibleItems() {
		visible[it.(skillItem).s.Dir] = true
	}
	for dir := range m.marked {
		if !visible[dir] {
			delete(m.marked, dir)
		}
	}
}

func (m *Tab) openBatch() {
	m.reconcileMarks()
	if len(m.marked) == 0 {
		m.setToast("Mark skills with m or select visible rows with v before opening batch actions", false)
		return
	}
	draft := batchDraft{visible: len(m.list.VisibleItems()), total: len(m.skills)}
	for _, it := range m.list.VisibleItems() {
		sk := it.(skillItem).s
		if m.marked[sk.Dir] {
			if sk.Origin != nil {
				o := *sk.Origin
				sk.Origin = &o
			}
			draft.plan.skills = append(draft.plan.skills, sk)
		}
	}
	draft.plan.agents = slices.Clone(m.agents)
	for i := range draft.plan.agents {
		draft.plan.agents[i].ReadDirs = slices.Clone(draft.plan.agents[i].ReadDirs)
	}
	if m.col < len(m.targets) {
		draft.plan.ids = []string{m.targets[m.col].ID}
	}
	m.batch, m.mode, m.toast = draft, skModeBatch, ""
}

func (m Tab) updateBatch(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	p := &m.batch.plan
	targets := skillCapableAgents(p.agents)
	last := len(targets)
	if p.action >= batchUpdate {
		last = 0
	}
	switch msg.String() {
	case "esc":
		m.mode = skModeList
	case "up", "k":
		m.batch.row = max(0, m.batch.row-1)
	case "down", "j":
		m.batch.row = min(last, m.batch.row+1)
	case "a":
		if p.action >= batchUpdate {
			break
		}
		p.ids = []string{}
		for _, ag := range targets {
			p.ids = append(p.ids, ag.ID)
		}
		m.batch.err = ""
	case "x":
		if p.action <= batchDisable {
			p.ids = []string{}
			m.batch.err = ""
		}
	case "left", "h", "right", "l", "space":
		if m.batch.row == 0 {
			delta := 1
			if msg.String() == "left" || msg.String() == "h" {
				delta = -1
			}
			p.action = batchAction((int(p.action) + delta + 4) % 4)
		} else {
			id := targets[m.batch.row-1].ID
			if slices.Contains(p.ids, id) {
				p.ids = slices.DeleteFunc(p.ids, func(v string) bool { return v == id })
			} else {
				p.ids = append(p.ids, id)
			}
		}
		m.batch.err = ""
	case "enter":
		if p.action <= batchDisable && len(p.ids) == 0 {
			m.batch.err = "Choose at least one agent"
			return m, nil
		}
		lines := []string{
			fmt.Sprintf("%s %d selected skill(s)?", p.action, len(p.skills)),
			fmt.Sprintf("Scope: %d selected / %d visible / %d total. Only the listed folders are included.", len(p.skills), m.batch.visible, m.batch.total),
		}
		if p.action <= batchDisable {
			lines = append(lines, "Agent destinations: "+m.agentLabels(p.ids), "Other activation states are preserved where native shared readers allow it; shared links may move. Local/foreign content is preserved.")
		} else {
			lines = append(lines, "Library operation: affects every agent reading these library copies.")
			if p.action == batchUpdate {
				lines = append(lines, "Fetch the recorded Git sources. Skip local edits, missing baselines and non-Git skills. Back up before replacement.")
			} else {
				lines = append(lines, "Back up, remove managed agent links and delete the library copies. Local/foreign copies are preserved.")
			}
		}
		for _, sk := range p.skills {
			lines = append(lines, "- "+sk.Dir+" ("+sk.Name+")")
			if p.action == batchUpdate && sk.Origin != nil && sk.Origin.Type == "git" {
				lines = append(lines, "  Source: "+sk.Origin.Source+" (subfolder: "+sk.Origin.Sub+")")
			}
		}
		m.confirm = components.NewConfirm(strings.Join(lines, "\n"))
		m.ckind, m.mode = confirmKindBatch, skModeConfirm
	}
	return m, nil
}

func (m Tab) batchView() string {
	panel := components.Panel{Title: "Selected skill actions", Focused: true, Width: min(m.width, 84)}
	inner := panel.ContentWidth()
	p := m.batch.plan
	lines := []string{
		ansi.Truncate(fmt.Sprintf("%d selected / %d visible", len(p.skills), m.batch.visible), inner, "…"),
		ansi.Truncate(fmt.Sprintf("%d total in inventory", m.batch.total), inner, "…"),
	}
	rows := []string{"Action: " + p.action.String()}
	if p.action <= batchDisable {
		for _, ag := range skillCapableAgents(p.agents) {
			mark := "[ ] "
			if slices.Contains(p.ids, ag.ID) {
				mark = "[x] "
			}
			rows = append(rows, mark+ag.Name)
		}
	} else {
		lines = append(lines, ansi.Truncate("Library copies; affects all their agent readers", inner, "…"))
	}
	footer := kit.Wrap("↑↓ field · ←→/space change\na all agents · x none · enter review\nesc cancel", inner, "")
	if p.action >= batchUpdate {
		footer = kit.Wrap("←→/space action · enter review\nesc cancel", inner, "")
	}
	if m.batch.err != "" {
		footer = kit.Wrap(m.batch.err, inner, "") + "\n" + footer
	}
	start, end := kit.Window(m.batch.row, len(rows), max(1, m.height-2-len(lines)-lipgloss.Height(footer)))
	for i := start; i < end; i++ {
		prefix := "  "
		if i == m.batch.row {
			prefix = "▸ "
		}
		line := ansi.Truncate(prefix+rows[i], inner, "…")
		if i == m.batch.row {
			line = kit.StTitle.Render(line)
		}
		lines = append(lines, line)
	}
	return panel.Render(strings.Join(append(lines, footer), "\n"))
}

func (m *Tab) finishBatch(msg skillBatchMsg) {
	m.inFlight, m.mode, m.toast = false, skModeList, ""
	if msg.plan.action >= batchUpdate {
		m.updateStatus = nil
	}
	svc, plan := m.svc, msg.plan
	m.report = &components.OperationReport{
		Title: fmt.Sprintf("%s %d selected skills", plan.action, len(plan.skills)), Result: msg.result,
		Recovery: "B lists skill archives. Restoring files does not restore removed agent links. Re-enable them in the matrix. Successful and skipped pairs are not retried.",
		Recover:  func() tea.Msg { return openBackupsMsg{} },
		Retry:    func(ids []string) core.OperationResult { return svc.runBatch(plan, ids) },
	}
}
