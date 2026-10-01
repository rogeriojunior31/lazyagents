package providers

import (
	"fmt"
	"net/url"
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

// inUseHeight is the "IN USE" block: title, one agent per line and a gap.
func (m Tab) inUseHeight() int { return min(2+len(m.statuses), max(0, m.bodyHeight()-4)) }

// split divides what is left below "IN USE" between the profile table and
// the detail; when stacked, height the table does not use goes to the detail.
func (m Tab) split() kit.Split {
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
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.form != nil {
		return m.form.view(m.width, m.height)
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  reading profiles and configs…")
	}
	if len(m.statuses) == 0 {
		return kit.StHint.Render("  no installed agent supports switching providers")
	}

	sp := m.split()
	table := m.profilesView(sp.ListW, sp.ListH)
	detail := kit.DetailView(sp, m.detailTitle(), m.detailViewport(sp))
	body := lipgloss.JoinVertical(lipgloss.Left, table, detail)
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", detail)
	}
	hints := kit.Hints(m.width, [2]string{"n", "new profile"}, [2]string{"x", "back to default"}, [2]string{"?", "help"})
	if len(m.profiles) > 0 {
		hints = kit.Hints(m.width, [2]string{"space", "apply/clear"}, [2]string{"←→", "agent"},
			[2]string{"a", "all"}, [2]string{"n", "new"}, [2]string{"e", "edit"},
			[2]string{"x", "back to default"}, [2]string{"?", "help"})
	}
	out := []string{kit.Frame(m.inUseView(m.width), "", m.inUseHeight()), body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// inUseView is what each agent uses now, the question that brings users here.
func (m Tab) inUseView(w int) string {
	nameW := 6
	for _, st := range m.statuses {
		nameW = max(nameW, 2+lipgloss.Width(st.AgentName))
	}
	cols := []kit.Column{{Width: nameW}, {Flex: true}}
	lines := []string{"  " + kit.StTitle.Render("IN USE")}
	for _, st := range m.statuses {
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render("● " + st.AgentName)
		mark, label := agentState(st, agent.ProviderProfile{}, false)
		if st.Active {
			label += kit.StHint.Render(" · " + endpointHost(st.Applied.BaseURL))
		}
		lines = append(lines, kit.TableRow(w, false, cols, name, mark+" "+label))
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
	nameW, modelW := 6, 6
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
			kit.StHint.Render("  No profiles yet — ")+components.Keycap("n")+kit.StHint.Render(" creates one here, or from the CLI:"),
			kit.CardValue.Render("  lazyagents provider add work --base-url https://… --token -"))
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
	return "FILES"
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
	if ok {
		b.WriteString(field("endpoint", orDash(pr.BaseURL), inner))
		b.WriteString(field("model", orDefault(pr.Model, "agent default"), inner))
		b.WriteString(field("token", tokenLabel(pr), inner))
		if pr.WireAPI != "" {
			b.WriteString(field("wire api", kit.CardValue.Render(pr.WireAPI)+kit.StHint.Render("  (Codex, Pi)"), inner))
		}
		b.WriteString("\n")
	}
	indent := strings.Repeat(" ", 4)
	nameW := 0
	for _, st := range m.statuses {
		nameW = max(nameW, lipgloss.Width(st.AgentName))
	}
	for i, st := range m.statuses {
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
		// With the selected profile applied, this line would repeat the card above.
		if cur := currentLine(st); cur != "" && !(ok && st.Profile == pr.Name) {
			b.WriteString(kit.Wrap(cur, inner, indent) + "\n")
		}
		b.WriteString(kit.Wrap(kit.StHint.Render(core.Tilde(st.File, m.svc.home)), inner, indent) + "\n")
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
	case selected && st.Profile == pr.Name:
		return kit.StOn.Render("●"), kit.StOn.Render("applied")
	case st.Active && st.Profile != "":
		return kit.StShared.Render("◆"), kit.StShared.Render(fmt.Sprintf("uses profile %s", st.Profile))
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
	pad := strings.Repeat(" ", 10)
	wrapped := kit.Wrap(value, inner, pad)
	return kit.CardLabel.Render(fmt.Sprintf("%-10s", label)) + strings.TrimPrefix(wrapped, pad) + "\n"
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
