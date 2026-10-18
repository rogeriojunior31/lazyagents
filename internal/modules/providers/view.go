package providers

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// bodyHeight is the body height minus hints and toast.
func (m Tab) bodyHeight() int { return max(6, m.height-2) }

// inUseHeight is the "IN USE" block: title, header, one agent per line and a gap.
func (m Tab) inUseHeight() int {
	if m.compactDiagnostics() {
		return 3 // Header and selected agent; leave room for actionable evidence.
	}
	return min(3+len(m.statuses), max(0, m.bodyHeight()-4))
}

func (m Tab) compactDiagnostics() bool {
	if m.height >= 20 {
		return false
	}
	for _, st := range m.statuses {
		if len(st.Environment) > 0 {
			return true
		}
	}
	return false
}

// split divides what is left below "IN USE" between the profile table and
// the detail; when stacked, height the table does not use goes to the detail.
func (m Tab) split() kit.Split {
	if m.compactDiagnostics() {
		return kit.Split{DetailW: m.width, DetailH: max(1, m.bodyHeight()-m.inUseHeight())}
	}
	sp := kit.SplitDetail(m.width, m.bodyHeight()-m.inUseHeight())
	if !sp.Side {
		need := profilesTop + max(1, len(m.profiles)) + 1
		if spare := sp.ListH - need; spare > 0 {
			sp.ListH -= spare
			sp.DetailH += spare
		}
	}
	return sp
}

// View clamps everything to the tab width: a safety net for narrow terminals.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if m.writing {
		return kit.StHint.Render("  Applying reviewed changes…")
	}
	if m.connection != nil {
		return m.connectionView()
	}
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.templates != nil {
		return m.templatesView()
	}
	if m.targets != nil {
		return m.targetsView()
	}
	if m.preparing {
		return kit.StHint.Render("  Checking selected agents… esc cancels")
	}
	if m.report != nil {
		return m.report.View(m.width, m.height)
	}
	if m.form != nil {
		return m.form.view(m.width, m.height)
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  reading profiles and configs…")
	}
	if len(m.statuses) == 0 {
		return kit.Wrap("No provider-capable agents detected. Press n to create a library profile. Install a compatible CLI and run : agents refresh before applying it.", m.width, "  ")
	}

	var body string
	showDetail := len(m.profiles) > 0
	for _, st := range m.statuses {
		showDetail = showDetail || len(st.Environment) > 0
	}
	if m.compactDiagnostics() {
		sp := m.split()
		body = kit.DetailView(sp, m.detailTitle(), m.detailViewport(sp))
	} else if !showDetail {
		body = m.profilesView(m.width, m.bodyHeight()-m.inUseHeight())
	} else {
		sp := m.split()
		table := m.profilesView(sp.ListW, sp.ListH)
		detail := kit.DetailView(sp, m.detailTitle(), m.detailViewport(sp))
		body = lipgloss.JoinVertical(lipgloss.Left, table, detail)
		if sp.Side {
			body = lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", detail)
		}
	}
	hints := kit.Hints(m.width, [2]string{"n", "new profile"}, [2]string{"←→", "agent"}, [2]string{"i", "import agent"}, [2]string{"x", "back to default"}, [2]string{"?", "help"})
	if showDetail && len(m.profiles) == 0 {
		hints = kit.Hints(m.width, [2]string{"←→", "agent"}, [2]string{"ctrl+u/d", "scroll details"}, [2]string{"n", "new profile"}, [2]string{"?", "help"})
	}
	if len(m.profiles) > 0 {
		hints = kit.Hints(m.width, [2]string{"space", "apply/clear"}, [2]string{"←→", "agent"},
			[2]string{"p", "preview/reapply"}, [2]string{"a", "choose agents"}, [2]string{"n", "new"}, [2]string{"i", "import"}, [2]string{"e", "edit"},
			[2]string{"c", "check URL"}, [2]string{"m", "models"}, [2]string{"?", "help"})
	}
	out := []string{kit.Frame(m.inUseView(m.width), "", m.inUseHeight()), body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// inUseWide is the width from which "in use" also shows the endpoint and the
// file each agent's provider lives in, as its own columns.
const inUseWide = 100

// inUseView is what each agent uses now, the question that brings users here,
// and the file lazyagents rewrites for it.
func (m Tab) inUseView(w int) string {
	nameW, stateW := len("agent"), len("provider")
	for _, st := range m.statuses {
		nameW = max(nameW, 2+lipgloss.Width(st.AgentName))
		_, label := agentState(st, agent.ProviderProfile{}, false)
		stateW = max(stateW, 2+lipgloss.Width(label))
	}
	wide := w >= inUseWide
	cols := []kit.Column{{Title: "agent", Width: nameW}, {Title: "provider", Flex: true}}
	if wide {
		cols = []kit.Column{{Title: "agent", Width: nameW}, {Title: "provider", Width: min(stateW, 40)},
			{Title: "endpoint", Width: 32}, {Title: "file", Flex: true}}
	}
	lines := []string{"  " + kit.StTitle.Render("IN USE"), kit.TableHeader(w, cols)}
	for i, st := range m.statuses {
		if m.compactDiagnostics() && i != m.col {
			continue
		}
		prefix := "● "
		if i == m.col {
			prefix = "▸ "
		}
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(prefix + st.AgentName)
		mark, label := agentState(st, agent.ProviderProfile{}, false)
		host := ""
		if st.Active {
			host = endpointHost(st.Applied.BaseURL)
		}
		if !wide {
			if host != "" {
				label += kit.StHint.Render(" · " + host)
			}
			lines = append(lines, kit.TableRow(w, false, cols, name, mark+" "+label))
			continue
		}
		if host == "" {
			host = kit.StHint.Render("—")
		}
		lines = append(lines, kit.TableRow(w, false, cols, name, mark+" "+label, host,
			kit.StHint.Render(core.Tilde(st.File, m.svc.home))))
	}
	return strings.Join(lines, "\n")
}

// profilesTop is the title plus the profile table header row.
const profilesTop = 2

// Profile table columns; agents start at colAgents.
const (
	colName = iota
	colHost
	colModel
	colAgents
)

// statusAgents converts the tab's agents to the agent column format.
func (m Tab) statusAgents() []agent.Agent {
	ags := make([]agent.Agent, len(m.statuses))
	for i, st := range m.statuses {
		ags[i] = agent.Agent{ID: st.AgentID, Name: st.AgentName, Short: st.Short}
	}
	return ags
}

// tableCols: profile, endpoint host (flex), model and one column per agent,
// with the cursor column underlined.
func (m Tab) tableCols(width int) []kit.Column {
	agents := kit.AgentColumns(m.statusAgents(), width/3)
	for i, st := range m.statuses {
		if i == m.col {
			agents[i].Title = lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Underline(true).Bold(true).
				Render(ansi.Strip(agents[i].Title))
		}
	}
	nameW, modelW := len("profile"), len("model")
	for _, p := range m.profiles {
		nameW = max(nameW, lipgloss.Width(p.Name))
		modelW = max(modelW, lipgloss.Width(p.Model))
	}
	cols := []kit.Column{
		{Title: "profile", Width: min(nameW, 20)},
		{Title: "endpoint", Flex: true},
		{Title: "model", Width: min(modelW, 20)},
	}
	if width < 64 {
		cols[colModel] = kit.Column{}
	}
	return append(cols, agents...)
}

// cells is a profile row: ● where applied. On the selected row the cell under
// the cursor is inverted: that is what space toggles.
func (m Tab) cells(p agent.ProviderProfile, selected bool) []string {
	out := []string{p.Name, kit.StHint.Render(endpointHost(p.BaseURL)), kit.StHint.Render(p.Model)}
	for i, st := range m.statuses {
		mark := kit.StOff.Render("○")
		switch {
		case !st.Installed:
			mark = kit.StOff.Render("–")
		case slices.Contains(st.Matches, p.Name) && (len(st.Matches) > 1 || st.External):
			mark = kit.StWarn.Render("◆")
		case st.Profile == p.Name:
			mark = kit.StOn.Render("●")
		}
		if selected && i == m.col {
			mark = lipgloss.NewStyle().Reverse(true).Render(ansi.Strip(mark))
		}
		out = append(out, mark)
	}
	return out
}

func (m Tab) profilesView(w, h int) string {
	title := kit.StTitle.Render("PROFILES") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.profiles)))
	cols := m.tableCols(w)
	lines := []string{"  " + title, kit.TableHeader(w, cols)}
	if len(m.profiles) == 0 {
		lines = append(lines[:1],
			kit.Wrap("No saved profiles. Press n to choose a template or create a profile.", w, "  "),
			kit.Wrap("Press i to import the selected agent's public settings. Saving stays in the library; review and apply separately.", w, "  "))
	}
	start, end := kit.Window(m.cursor, len(m.profiles), max(1, h-profilesTop))
	for i := start; i < end; i++ {
		lines = append(lines, kit.TableRow(w, i == m.cursor, cols, m.cells(m.profiles[i], i == m.cursor)...))
	}
	return kit.Frame(strings.Join(lines, "\n"), "", h)
}

// endpointHost shortens the URL to its host, which is what tells providers
// apart in a narrow row.
func endpointHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func (m Tab) detailTitle() string {
	if p, ok := m.current(); ok {
		return strings.ToUpper(p.Name)
	}
	return "LIVE SETTINGS"
}

// detailViewport builds the detail viewport at the current scroll position.
func (m Tab) detailViewport(sp kit.Split) viewport.Model {
	w, h := kit.DetailSize(sp)
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(ansi.Wrap(m.detailContent(w), w, ""))
	vp.SetYOffset(m.detailOff)
	return vp
}

func (m *Tab) scrollDetail(msg tea.Msg) {
	vp, _ := m.detailViewport(m.split()).Update(msg)
	m.detailOff = vp.YOffset()
}

// detailContent is the whole profile (full endpoint, model, token) and, per
// agent, the file that apply rewrites and what it holds today.
func (m Tab) detailContent(inner int) string {
	var b strings.Builder
	pr, ok := m.current()
	if ok && !m.compactDiagnostics() {
		b.WriteString(field("endpoint", orDash(pr.BaseURL), inner))
		b.WriteString(field("model", orDefault(pr.Model, "agent default"), inner))
		b.WriteString(field("token", tokenLabel(pr), inner))
		if pr.EnvKey != "" && (pr.HasToken || pr.Token != "") {
			b.WriteString(field("env var", kit.CardValue.Render("$"+pr.EnvKey), inner))
		}
		if pr.WireAPI != "" {
			b.WriteString(field("wire api", kit.CardValue.Render(pr.WireAPI)+kit.StHint.Render("  (agent-specific)"), inner))
		}
		b.WriteString("\n")
	}
	indent := strings.Repeat(" ", 4)
	nameW := 0
	for _, st := range m.statuses {
		nameW = max(nameW, lipgloss.Width(st.AgentName))
	}
	for i, st := range m.statuses {
		if m.compactDiagnostics() && i != m.col {
			continue
		}
		mark, label := agentState(st, pr, ok)
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", nameW, st.AgentName))
		cursor := ""
		if ok {
			cursor = "  "
			if i == m.col {
				cursor = kit.StTitle.Render("▸ ") // the agent space applies
			}
		}
		b.WriteString(fmt.Sprintf("%s%s %s  %s\n", cursor, mark, name, label))
		for _, diagnostic := range st.Environment {
			label := "Environment"
			if diagnostic.Variable != "" {
				label += " (" + diagnostic.Variable + ")"
			}
			b.WriteString(kit.Wrap(kit.StWarn.Render(label)+": "+diagnostic.Explanation, inner, indent) + "\n")
			b.WriteString(kit.Wrap("Source: "+diagnostic.Source, inner, indent) + "\n")
			if diagnostic.OtherSource != "" {
				b.WriteString(kit.Wrap("Compared with: "+diagnostic.OtherSource, inner, indent) + "\n")
			}
			b.WriteString(kit.Wrap("Next: "+diagnostic.NextStep, inner, indent) + "\n")
		}
		if cur := currentLine(st); cur != "" {
			b.WriteString(kit.Wrap("Live: "+cur, inner, indent) + "\n")
		} else if st.Installed && st.Err == "" {
			b.WriteString(kit.Wrap("Live: native settings; no explicit provider snapshot", inner, indent) + "\n")
		}
		if len(st.Matches) > 1 {
			b.WriteString(kit.Wrap("Matching profiles: "+strings.Join(st.Matches, ", "), inner, indent) + "\n")
		}

		if comparison, found := st.comparison(pr.Name); ok && found && i == m.col {
			for _, diff := range comparison.Differences {
				line := fmt.Sprintf("%s: saved %s; live %s", diff.Field, comparisonValue(diff.Saved), comparisonValue(diff.Live))
				b.WriteString(kit.Wrap(line, inner, indent) + "\n")
			}
			if len(comparison.Inherited) > 0 {
				b.WriteString(kit.Wrap("Unspecified fields use native apply/clear behavior: "+strings.Join(comparison.Inherited, ", "), inner, indent) + "\n")
			}
			b.WriteString(kit.Wrap("Token values are not compared. Keep live: leave unchanged. Reapply: p (preview). Refresh: r.", inner, indent) + "\n")
		}

		if m.width < inUseWide { // wide, "in use" shows the file
			b.WriteString(kit.Wrap(kit.StHint.Render(core.Tilde(st.File, m.svc.home)), inner, indent) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// agentState is the agent's marker and label relative to the selected profile
// (or just its state, without a profile).
func agentState(st Status, pr agent.ProviderProfile, selected bool) (string, string) {
	switch {
	case st.Err != "":
		return kit.StErr.Render("!"), kit.StErr.Render(st.Err)
	case !st.Installed:
		return kit.StOff.Render("–"), kit.StOff.Render("CLI not installed")
	case len(st.Environment) > 0:
		return kit.StWarn.Render("!"), kit.StWarn.Render("environment needs review; see details")
	case !st.Active:
		return kit.StOff.Render("○"), kit.StOff.Render("agent default")
	case selected && comparisonDiffers(st, pr.Name):
		return kit.StWarn.Render("◆"), kit.StWarn.Render("saved settings differ; apply to update")
	case len(st.Matches) > 1:
		return kit.StWarn.Render("◆"), kit.StWarn.Render(fmt.Sprintf("ambiguous: %d profiles match; choose one to reapply", len(st.Matches)))
	case selected && st.Profile == pr.Name && st.External:
		return kit.StWarn.Render("◆"), kit.StWarn.Render("matches saved fields; apply to manage")
	case selected && st.Profile == pr.Name:
		return kit.StOn.Render("●"), kit.StOn.Render("matches saved public fields")
	case st.Active && st.SavedChanged:
		return kit.StWarn.Render("◆"), kit.StWarn.Render("saved settings differ; no profile match")
	case st.Active && st.Profile != "" && st.External:
		return kit.StWarn.Render("◆"), kit.StWarn.Render(fmt.Sprintf("external settings match profile %s", st.Profile))
	case st.Active && st.Profile != "":
		return kit.StShared.Render("◆"), kit.StShared.Render(fmt.Sprintf("matches profile %s", st.Profile))
	case st.Active:
		return kit.StWarn.Render("◆"), kit.StWarn.Render("provider set outside lazyagents")
	}
	return kit.StOff.Render("○"), kit.StOff.Render("agent default")
}

// currentLine describes the provider active in the agent, always from the
// redacted read of the live config.
func currentLine(st Status) string {
	if !st.Active {
		return ""
	}
	a := st.Applied
	var parts []string
	if a.BaseURL != "" {
		parts = append(parts, kit.CardValue.Render(a.BaseURL))
	}
	if a.WireAPI != "" {
		parts = append(parts, kit.CardLabel.Render("protocol ")+kit.CardValue.Render(strconv.Quote(a.WireAPI)))
	}
	if a.Model != "" {
		parts = append(parts, kit.CardLabel.Render("model ")+kit.CardValue.Render(a.Model))
	}
	switch {
	case a.EnvKey != "":
		parts = append(parts, kit.CardLabel.Render("token in $"+a.EnvKey))
	case a.HasToken:
		parts = append(parts, kit.StOn.Render("token ✓"))
	}
	return strings.Join(parts, kit.StHint.Render(" · "))
}

// field is a label with an aligned value; a long value (endpoint) wraps at
// the value column without losing its end.
func field(label, value string, inner int) string {
	return kit.Field(label, value, 10, inner) + "\n"
}

func orDash(s string) string { return orDefault(s, "—") }

func orDefault(s, def string) string {
	if s == "" {
		return kit.StHint.Render(def)
	}
	return kit.CardValue.Render(s)
}

// tokenLabel says whether there is a token and where, never its value.
func tokenLabel(p agent.ProviderProfile) string {
	switch {
	case p.EnvKey != "" && p.HasToken:
		return kit.StOn.Render("saved") + kit.StHint.Render(" · env $"+p.EnvKey)
	case p.EnvKey != "":
		return kit.CardValue.Render("$" + p.EnvKey)
	case p.HasToken:
		return kit.StOn.Render("saved") + kit.StHint.Render(" · masked")
	}
	return kit.StHint.Render("no token")
}

func comparisonDiffers(st Status, name string) bool {
	comparison, ok := st.comparison(name)
	return ok && len(comparison.Differences) > 0
}

func comparisonValue(value string) string {
	if value == "" {
		return "native default / unset"
	}
	return strconv.Quote(value)
}
