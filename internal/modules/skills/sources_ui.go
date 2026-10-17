package skills

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type sourceScanStatus struct {
	at                time.Time
	count             int
	revision, failure string
}
type sourceBrowser struct {
	items          []repository
	cursor, offset int
	status         map[repository]sourceScanStatus
	form           repository
	edit           *repository
	field          int
	err            string
}
type openSourcesMsg struct{}
type sourcesLoadedMsg struct {
	items []repository
	err   error
}
type sourceSavedMsg struct{ err error }

func (m Tab) loadSourcesCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg { items, err := svc.repositories(); return sourcesLoadedMsg{items, err} }
}

func (m *Tab) sourceForm(edit *repository) tea.Cmd {
	m.sources.form, m.sources.edit, m.sources.field, m.sources.err = repository{}, edit, 0, ""
	if edit != nil {
		m.sources.form = *edit
		m.sources.field = 1
	}
	m.mode = skModeSourceForm
	return m.sourceInput()
}

func (m *Tab) sourceInput() tea.Cmd {
	fields := []string{m.sources.form.Name, m.sources.form.URL, m.sources.form.Ref, m.sources.form.Subdir}
	m.input.SetValue(fields[m.sources.field])
	m.input.Placeholder = []string{"ECC (or another source name)", "owner/repo or Git URL", "branch or tag; empty uses the remote default", "folder inside the repository; empty uses its root"}[m.sources.field]
	return m.input.Focus()
}

func (m Tab) updateSourceForm(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeSources
		m.sources.err = ""
		m.input.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		p := &m.sources.form
		switch m.sources.field {
		case 0:
			p.Name = value
		case 1:
			p.URL = value
		case 2:
			p.Ref = value
		case 3:
			p.Subdir = value
		}
		if m.sources.field < 3 {
			m.sources.field++
			return m, m.sourceInput()
		}
		normalized, err := normalizeRepository(*p)
		if err != nil {
			m.sources.err = err.Error()
			m.sources.field = 0
			if m.sources.edit != nil {
				m.sources.field = 1
			}
			return m, m.sourceInput()
		}
		svc, expected := m.svc, m.sources.edit
		m.input.Blur()
		m.mode = skModeSourceBusy
		return m, tea.Batch(m.beginSpin("saving skill source…"), func() tea.Msg { return sourceSavedMsg{svc.saveRepository(normalized, expected, false)} })
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Tab) updateSources(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	if kit.DetailScroll[msg.String()] {
		vp := m.sourceViewport()
		vp, _ = vp.Update(kit.DetailScrollMsg(msg))
		m.sources.offset = vp.YOffset()
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = skModeList
	case "a":
		return m, m.sourceForm(nil)
	case "r":
		return m, m.loadSourcesCmd()
	case "up", "k":
		m.sources.cursor = max(0, m.sources.cursor-1)
		m.sources.offset = 0
	case "down", "j":
		m.sources.cursor = min(max(0, len(m.sources.items)-1), m.sources.cursor+1)
		m.sources.offset = 0
	case "enter", "s", "e", "d":
		if len(m.sources.items) == 0 {
			return m, nil
		}
		r := m.sources.items[m.sources.cursor]
		switch msg.String() {
		case "e":
			return m, m.sourceForm(&r)
		case "d":
			m.sources.edit = &r
			m.ckind = confirmKindDeleteSource
			m.confirm = components.NewConfirm(fmt.Sprintf("Remove source %q from the repository list?\nInstalled skills, their recorded update origins and agent links are kept.\n%s", r.Name, r.URL))
			m.mode = skModeConfirm
		default:
			m.mode = skModeSourceBusy
			return m, tea.Batch(m.beginSpin(fmt.Sprintf("scanning source %s…", r.Name)), m.repositoryCmd(r))
		}
	}
	return m, nil
}

func (m Tab) repositoryCmd(r repository) tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		found, origin, cleanup, err := svc.scanRepository(r)
		ids, prefErr := svc.installTargets()
		if prefErr != nil {
			origin.Notes = append(origin.Notes, fmt.Sprintf("Could not load install destinations; using library only: %s", prefErr))
		}
		return discoverMsg{found: found, origin: origin, cleanup: cleanup, err: err, targets: ids, repository: &r}
	}
}

func (m Tab) sourceHints() string {
	return kit.Wrap("enter scan · a add · e edit\nd remove · r reload · esc back\nshift+↑↓ scroll details", max(1, m.width-4), "")
}

func (m Tab) sourceWindow() (int, int) {
	return kit.Window(m.sources.cursor, len(m.sources.items), max(1, (m.height-6)/3))
}

func (m Tab) sourceViewport() viewport.Model {
	start, end := m.sourceWindow()
	w := max(1, m.width-4)
	h := max(1, m.height-2-max(1, end-start)-lipgloss.Height(m.sourceHints()))
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	text := "No repositories registered. Add an optional source such as ECC with a. Nothing is scanned or downloaded at startup."
	if len(m.sources.items) > 0 {
		r := m.sources.items[m.sources.cursor]
		status := m.sources.status[r]
		ref, sub := r.Ref, r.Subdir
		if ref == "" {
			ref = "Remote default branch"
		}
		if sub == "" {
			sub = "Repository root"
		}
		lines := []string{r.Name, "URL: " + r.URL, "Ref: " + ref, "Subfolder: " + sub, "Last scan: not scanned in this session"}
		if !status.at.IsZero() {
			lines[4] = "Last scan: " + status.at.Local().Format("2006-01-02 15:04:05")
			if status.failure != "" {
				lines = append(lines, "Error: "+status.failure)
			} else {
				lines = append(lines, fmt.Sprintf("%d entries found; commit %s", status.count, status.revision))
			}
		}
		lines = append(lines, "Scan runs only on request. Editing/removing this descriptor never changes installed skill origins.")
		text = strings.Join(lines, "\n")
	}
	if m.sources.err != "" {
		text = "Source error: " + m.sources.err + "\n" + text
	}
	vp.SetContent(kit.Wrap(text, w, ""))
	vp.SetYOffset(m.sources.offset)
	return vp
}

func (m Tab) sourcesView() string {
	lines := []string{}
	start, end := m.sourceWindow()
	for i := start; i < end; i++ {
		r := m.sources.items[i]
		state := "not scanned"
		status := m.sources.status[r]
		if !status.at.IsZero() {
			state = "scanned"
			if status.failure != "" {
				state = "scan failed"
			}
		}
		prefix := "  "
		if i == m.sources.cursor {
			prefix = "› "
		}
		line := ansi.Truncate(fmt.Sprintf("%s%s · %s", prefix, r.Name, state), max(1, m.width-4), "…")
		if i == m.sources.cursor {
			line = kit.StTitle.Render(line)
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = append(lines, "No saved sources")
	}
	lines = append(lines, m.sourceViewport().View(), m.sourceHints())
	return components.Panel{Title: fmt.Sprintf("Skill sources · %d", len(m.sources.items)), Focused: true, Width: m.width}.Render(strings.Join(lines, "\n"))
}

func (m Tab) sourceFormView() string {
	prompts := []string{"Source name (unique; unchanged when editing):", "Git URL or owner/repo:", "Branch/tag (verified on scan; empty = default):", "Subfolder (empty = whole repository):"}
	text := prompts[m.sources.field]
	if m.sources.err != "" {
		text = m.sources.err + "\n" + text
	}
	w := min(m.width, 72)
	inner := max(1, w-4)
	hints := kit.Wrap("enter next/save · esc cancel", inner, "")
	prompt := kit.Wrap(text, inner, "")
	vp := viewport.New(viewport.WithWidth(inner), viewport.WithHeight(min(lipgloss.Height(prompt), max(1, m.height-5-lipgloss.Height(hints)))))
	vp.SetContent(prompt)
	content := lipgloss.JoinVertical(lipgloss.Left, vp.View(), "", components.InputView(m.input, inner), "", hints)
	return components.Panel{Title: fmt.Sprintf("Source · %d/4", m.sources.field+1), Focused: true, Width: w}.Render(content)
}
