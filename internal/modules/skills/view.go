package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

func (m Tab) View() string {
	switch m.mode {
	case skModeInstall:
		return m.inputModal("Install skill",
			"Source (GitHub, local folder or .zip):",
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "search"}, [2]string{"esc", "back"}))
	case skModeNew:
		return m.inputModal("New skill",
			fmt.Sprintf("Name (becomes the folder in %s):", core.Tilde(m.svc.Paths().LibraryDir(), m.svc.Paths().Home)),
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "create/edit"}, [2]string{"esc", "back"}))
	case skModePick:
		return m.picker.view(m.width, m.height)
	case skModeRegistry:
		return m.inputModal("Search GitHub",
			"Search term (repositories with SKILL.md):",
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "search"}, [2]string{"esc", "back"}))
	case skModeRegistryPick:
		return m.regPicker.view(m.width, m.height)
	case skModeConfirm:
		return m.confirm.ViewIn(m.width, m.height)
	case skModeDoc:
		head := kit.StTitle.Render(m.docName) + kit.StHint.Render("  SKILL.md · e edit · esc back · ↑↓/mouse wheel scroll")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	case skModeBackup:
		return m.backupPicker.view(m.width, m.height)
	case skModeProfiles:
		return m.profilesView()
	case skModeProfileName:
		return m.inputModal("Save profile",
			"Profile name:",
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "save"}, [2]string{"esc", "back"}))
	}

	sp := m.split()
	hints := kit.Hints(m.width, [2]string{"space", "toggle"}, [2]string{"←→", "agent"},
		[2]string{"enter", "read"}, [2]string{"i", "install"}, [2]string{"p", "profiles"},
		[2]string{"/", "filter"}, [2]string{"?", "help"})
	body := lipgloss.JoinVertical(lipgloss.Left, m.tableView(sp.ListW, sp.ListH), m.detailView(sp))
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.tableView(sp.ListW, sp.ListH), "  ", m.detailView(sp))
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// split divides the body between matrix and detail (beside or below); when
// stacked, the height the matrix does not use goes to the detail.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight())
	if !sp.Side {
		need := len(m.tableHead(sp.ListW)) + max(1, len(m.list.VisibleItems())) + 1
		if spare := sp.ListH - need; spare > 0 {
			sp.ListH -= spare
			sp.DetailH += spare
		}
	}
	return sp
}

// legend explains the matrix markers and hides when it does not fit the title.
// Rendered on use, not at init, so colors follow the active theme.
func legend() string {
	return kit.StOn.Render("●") + kit.StHint.Render(" enabled  ") +
		kit.StLocal.Render("▪") + kit.StHint.Render(" local  ") +
		kit.StShared.Render("◆") + kit.StHint.Render(" via other agent  ") +
		kit.StOff.Render("○") + kit.StHint.Render(" disabled")
}

// tableHead are the lines above the skills: title with legend, filter (if
// any) and column names.
func (m Tab) tableHead(w int) []string {
	title := kit.StTitle.Render("LIBRARY") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.skills)))
	if n := len(m.localNames()); n > 0 {
		title += kit.StHint.Render(fmt.Sprintf(" · %d local", n))
	}
	if gap := w - 2 - lipgloss.Width(title) - lipgloss.Width(legend()); gap >= 2 {
		title += strings.Repeat(" ", gap) + legend()
	}
	lines := []string{"  " + title}
	if m.list.FilterState() != list.Unfiltered {
		m.list.FilterInput.SetWidth(max(1, w-6))
		lines = append(lines, "  "+m.list.FilterInput.View())
	}
	return append(lines, kit.TableHeader(w, m.tableCols(w)))
}

// tableWindow returns the visible skill range and the row of the first one;
// rendering and clicks share this math.
func (m Tab) tableWindow(w, h int) (start, end, top int) {
	top = len(m.tableHead(w))
	start, end = kit.Window(m.list.Index(), len(m.list.VisibleItems()), max(1, h-top))
	return start, end, top
}

// tableView draws the skill × agent matrix in w×h.
func (m Tab) tableView(w, h int) string {
	lines := m.tableHead(w)
	items := m.list.VisibleItems()
	switch {
	case len(items) == 0 && m.list.FilterState() == list.FilterApplied:
		lines = append(lines, kit.StHint.Render(fmt.Sprintf("  Nothing found for “%s”.", m.list.FilterValue())))
	case len(m.skills) == 0:
		lines = append(lines, kit.StHint.Render("  Library empty — ")+components.Keycap("i")+kit.StHint.Render(" install"))
	}
	cols := m.tableCols(w)
	start, end, _ := m.tableWindow(w, h)
	for i := start; i < end; i++ {
		if it, ok := items[i].(skillItem); ok {
			sel := i == m.list.Index()
			lines = append(lines, kit.TableRow(w, sel, cols, m.cells(it, sel)...))
		}
	}
	// No room in the title: the legend moves under the matrix if a line is left.
	foot := ""
	if !strings.Contains(lines[0], "disabled") && h-len(lines) >= 2 {
		foot = "  " + legend()
	}
	return kit.Frame(strings.Join(lines, "\n"), foot, h)
}

// refreshDetail recomputes the detail viewport, keeping the scroll unless the
// selected skill changed.
func (m *Tab) refreshDetail() {
	w, h := kit.DetailSize(m.split())
	m.detailVP.SetWidth(w)
	m.detailVP.SetHeight(h)
	name := ""
	if sel, ok := m.selected(); ok {
		name = sel.Name
	}
	if name != m.detailName {
		m.detailVP.GotoTop()
		m.detailName = name
	}
	m.detailVP.SetContent(m.detailContent(w))
}

// detailView shows the skill detail beside or below the matrix.
func (m Tab) detailView(sp kit.Split) string {
	return kit.DetailView(sp, "ABOUT THE SKILL", m.detailVP)
}

// detailContent builds the selected skill's card wrapped to inner columns
// (full description; the viewport scrolls).
func (m Tab) detailContent(inner int) string {
	sel, ok := m.selected()
	if !ok {
		// Line by line: a multi-line Render pads to the longest line and leaves
		// a gap before the keycap.
		return lipgloss.NewStyle().Width(inner).Render(lipgloss.JoinVertical(lipgloss.Left,
			kit.StText.Render("No skills here yet."),
			"",
			kit.StHint.Render("Press ")+components.Keycap("i")+
				kit.StHint.Render(" to install from GitHub, a folder or a zip.")))
	}
	home := m.svc.Paths().Home
	nameW := 0
	for _, ag := range m.targets {
		nameW = max(nameW, len(ag.Name))
	}

	var b strings.Builder
	b.WriteString(kit.StTitle.Render(sel.Name) + "\n")
	if sel.Description != "" {
		b.WriteString(clampLines(kit.StText.Render(sel.Description), inner, descLines) + "\n")
	}
	if sel.Warning != "" {
		b.WriteString(kit.StErr.Render("⚠ "+sel.Warning) + "\n")
	}
	for _, is := range m.selectedIssues() {
		b.WriteString(kit.StWarn.Render(fmt.Sprintf("! %s: %s", is.Field, is.Msg)) + "\n")
	}
	b.WriteString("\n" + kit.StHint.Render("AVAILABLE IN AGENTS") + "\n")
	for i, ag := range m.targets {
		st := sel.States[ag.ID]
		name := fmt.Sprintf("%-*s", nameW, ag.Name)
		var mark, status string
		switch {
		case st.On && st.Managed:
			mark, status = kit.StOn.Render("●"), kit.StOn.Render("enabled")
		case st.On && st.Local:
			mark, status = kit.StLocal.Render("▪"), kit.StLocal.Render("local · "+core.Tilde(st.Via, home))
		case st.On:
			mark, status = kit.StShared.Render("◆"), kit.StShared.Render("via "+core.Tilde(st.Via, home))
		default:
			mark, status = kit.StOff.Render("○"), kit.StOff.Render("disabled")
		}
		cursor := "  "
		if i == m.col {
			cursor = kit.StTitle.Render("▸ ") // the agent space toggles
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s  %s\n", cursor,
			components.Keycap(fmt.Sprintf("%d", i+1)), mark, kit.CardValue.Render(name), status))
	}
	b.WriteString("\n" + kit.StHint.Render("SOURCE") + "\n")
	if sel.InLibrary {
		b.WriteString(kit.CardLabel.Render("library     ") + kit.CardValue.Render(core.Tilde(sel.Path, home)) + "\n")
		if o := sel.Origin; o != nil {
			src := o.Source
			if o.Type != "git" {
				src = core.Tilde(src, home)
			}
			line := kit.CardLabel.Render("source      ") + kit.CardValue.Render(o.Type+" "+src)
			if !o.InstalledAt.IsZero() {
				line += kit.CardLabel.Render("  (" + o.InstalledAt.Format("2006-01-02") + ")")
			}
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString(kit.StLocal.Render("▪ outside the library — ") + components.Keycap("o") + kit.StLocal.Render(" adopt") + "\n")
	}
	return lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n"))
}

// descLines is how much of the description fits before the agents; the full
// text is in SKILL.md (enter).
const descLines = 4

// clampLines wraps text to width and cuts it at n lines, noting there is more.
func clampLines(text string, width, n int) string {
	lines := strings.Split(lipgloss.NewStyle().Width(width).Render(text), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n" +
		kit.StHint.Render("… ") + components.Keycap("enter") + kit.StHint.Render(" read the full SKILL.md")
}

// inputModal frames a text prompt (install/new/profile) in a Panel with the
// toast below; width is capped so it does not become an empty strip.
func (m Tab) inputModal(title, prompt, hint string) string {
	w := m.width
	if w > 72 {
		w = 72
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Width(max(1, w-4)).Render(prompt), "", components.InputView(m.input, w-4), "", hint)
	panel := components.Panel{Title: title, Focused: true, Width: w}.Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, "", m.toastLine())
}

func (m Tab) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.inFlight {
		return m.spin.View() + " " + kit.StHint.Render(m.toast)
	}
	return components.Toast(m.toast, m.toastErr)
}

// agentLabels turns agent ids into friendly names, comma-separated.
func (m Tab) agentLabels(ids []string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		label := id
		for _, ag := range m.agents {
			if ag.ID == id {
				label = ag.Name
				break
			}
		}
		names = append(names, label)
	}
	return strings.Join(names, ", ")
}
