package usage

import (
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// tabWith builds a loaded tab with events from two agents and projects.
func tabWith(t *testing.T, cfg config) *Tab {
	t.Helper()
	now := time.Now()
	evs := []agent.UsageEvent{
		{AgentID: "claude-code", Time: now.AddDate(0, 0, -20), CWD: "/home/u/antigo", Model: "claude-opus-4", Usage: agent.Usage{Input: 1000}},
		{AgentID: "claude-code", Time: now.Add(-time.Hour), CWD: "/home/u/alpha", Model: "claude-opus-4", Usage: agent.Usage{Input: 100}, N: 3},
		{AgentID: "codex", Time: now.Add(-30 * time.Minute), CWD: "/home/u/beta", Model: "gpt-6", Usage: agent.Usage{Input: 10}},
	}
	tab := newTab(New(nil, core.PathsIn(t.TempDir())), cfg)
	tab.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	tab.Update(statusMsg{statuses: []Status{{AgentID: "claude-code", AuthLabel: "subscription"}, {AgentID: "codex", AuthLabel: "subscription"}}})
	tab.Update(eventsMsg{events: evs})
	return &tab
}

func key(tab *Tab, k string) {
	code := []rune(k)[0]
	switch k {
	case "esc":
		code = tea.KeyEscape
	case "enter":
		code = tea.KeyEnter
	case "right":
		code = tea.KeyRight
	}
	msg := tea.KeyPressMsg{Code: code}
	if len([]rune(k)) == 1 {
		msg.Text = k
	}
	tab.Update(msg)
}

func screen(tab *Tab) string { return ansi.Strip(tab.View()) }

func TestTabFiltersPeriodAgentView(t *testing.T) {
	tab := tabWith(t, config{})
	if s := screen(tab); !strings.Contains(s, "Tokens per day · 7 days") || !strings.Contains(s, "110 tokens") {
		t.Fatalf("default should be 7 days by day, without the 20-day-old event:\n%s", s)
	}
	key(tab, "p") // 30 days: the old one comes in
	if s := screen(tab); !strings.Contains(s, "1.1k tokens") || !strings.Contains(s, "5 responses") {
		t.Errorf("30 days should sum everything (N counts responses):\n%s", s)
	}
	key(tab, "right")
	key(tab, "right") // project
	key(tab, "a")     // claude-code only
	s := screen(tab)
	if !strings.Contains(s, "Tokens per project · 30 days · claude-code") || !strings.Contains(s, "antigo") || strings.Contains(s, "beta") {
		t.Errorf("project + claude-code:\n%s", s)
	}
	key(tab, "esc") // no text: back to all agents
	if s := screen(tab); !strings.Contains(s, "beta") {
		t.Errorf("esc should go back to all agents:\n%s", s)
	}
}

func TestTabTextFilter(t *testing.T) {
	tab := tabWith(t, config{Period: "30d", View: "projects"})
	key(tab, "/")
	if !tab.Capturing() {
		t.Fatal("/ should open the input")
	}
	for _, r := range "ALP" {
		key(tab, string(r))
	}
	s := screen(tab)
	if !strings.Contains(s, "alpha") || strings.Contains(s, "beta") || strings.Contains(s, "antigo") {
		t.Errorf("text filter (case-insensitive):\n%s", s)
	}
	key(tab, "enter")
	if tab.Capturing() || tab.f.text != "ALP" {
		t.Errorf("enter should close and keep the filter: %+v", tab.f)
	}
	key(tab, "/")
	key(tab, "z")
	if s := screen(tab); !strings.Contains(s, `No row contains "ALPz"`) {
		t.Errorf("filter with no result:\n%s", s)
	}
	key(tab, "esc")
	if tab.Capturing() || tab.f.text != "" {
		t.Errorf("esc in the input should clear: %+v", tab.f)
	}
}

func TestTabConfigAndPalette(t *testing.T) {
	tab := tabWith(t, config{Period: "all", View: "models"})
	if s := screen(tab); !strings.Contains(s, "Tokens per model · all") {
		t.Errorf("config period/view not applied:\n%s", s)
	}
	if f := newFilters(config{Period: "xyz", View: "nada"}); f.period != 1 || f.view != 0 {
		t.Errorf("unknown value should fall back to the default: %+v", f)
	}
	var viewAgents tea.Msg
	for _, c := range tab.Commands() {
		if c.Name == "view agents" {
			viewAgents = c.Msg
		}
	}
	tab.Update(viewAgents)
	if s := screen(tab); !strings.Contains(s, "Tokens per agent") {
		t.Errorf("palette view agents:\n%s", s)
	}
}

// Scrolling stops at the end and the body is only rebuilt when it changes.
func TestTabScrollClampAndMemo(t *testing.T) {
	tab := tabWith(t, config{})
	for range 500 {
		key(tab, "j")
	}
	if max := max(0, len(tab.lines)-tab.contentHeight()); tab.scroll != max {
		t.Errorf("scroll = %d, want at most %d", tab.scroll, max)
	}
	before := tab.drawn
	key(tab, "k")
	if tab.drawn != before {
		t.Error("scrolling should not rebuild the body")
	}
}

func TestFiltersStayVisibleWhileScrolling(t *testing.T) {
	tab := tabWith(t, config{Period: "30d", View: "projects"})
	tab.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
	for range 100 {
		key(tab, "j")
	}
	view := screen(tab)
	if !strings.Contains(view, "30 days") || !strings.Contains(view, "project") || !strings.Contains(view, "help") || lipgloss.Height(tab.View()) > 11 {
		t.Fatalf("filters/actions off screen:\n%s", view)
	}
	before := tab.drawn
	key(tab, "k")
	if tab.drawn != before {
		t.Fatal("scrolling redid the aggregation")
	}
	key(tab, "/")
	tab.Update(tea.PasteMsg{Content: strings.Repeat("x", 50) + "FIM"})
	view = screen(tab)
	if !strings.Contains(view, "FIM") || !strings.Contains(view, "esc clears") || lipgloss.Height(tab.View()) > 11 {
		t.Fatalf("input cut off:\n%s", view)
	}
}

func TestUsageLongErrorsAndCachedLimitsRemainReadable(t *testing.T) {
	for _, width := range []int{36, 76, 116} {
		tab := tabWith(t, config{})
		tab.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		status := Status{AgentID: "codex", AuthLabel: "subscription", Cached: true, Err: strings.Repeat("failed to query the service ", 20) + "FINAL-DETAIL", Limits: agent.RateStatus{FetchedAt: time.Now().Add(-time.Hour), Windows: []agent.RateWindow{{Label: "Weekly window per model", UsedPercent: 93.4, ResetsAt: time.Now().Add(time.Hour)}}}}
		tab.Update(statusMsg{statuses: []Status{status}})
		var seen strings.Builder
		for range len(tab.lines) + 2 {
			view := tab.View()
			if lipgloss.Width(view) > width || lipgloss.Height(view) > 11 {
				t.Fatalf("notice off screen:\n%s", ansi.Strip(view))
			}
			seen.WriteString(ansi.Strip(view))
			seen.WriteByte('\n')
			key(tab, "j")
		}
		all := seen.String()
		for _, want := range []string{"FINAL-DETAIL", "previous limits kept", "93.4%", "resets", "r to retry"} {
			if !strings.Contains(all, want) {
				t.Errorf("information unreachable at %d columns: %s", width, want)
			}
		}
		tab.Update(statusMsg{statuses: []Status{{AgentID: "codex", Err: "NO-CACHE-END"}}})
		key(tab, "home")
		if !strings.Contains(ansi.Strip(tab.body()), "NO-CACHE-END") {
			t.Fatal("error without cache was cut off")
		}
	}
}

func TestUsageProgressWaitsForBothLoads(t *testing.T) {
	for _, limitsFirst := range []bool{false, true} {
		tab := tabWith(t, config{})
		tab.loadCmd(false) // only builds the commands, no I/O
		status := statusMsg{statuses: []Status{{AgentID: "codex", Err: "falhou"}}}
		if limitsFirst {
			tab.Update(status)
		} else {
			tab.Update(eventsMsg{})
		}
		if !tab.loading || !strings.Contains(screen(tab), "refreshing usage") {
			t.Fatal("the first response ended the progress")
		}
		if limitsFirst {
			tab.Update(eventsMsg{})
		} else {
			tab.Update(status)
		}
		if tab.loading || !tab.toastErr || !strings.Contains(tab.toast, "1 warning") {
			t.Fatal("the end of the query lost the notice")
		}
		tab.Update(statusMsg{})
		if tab.toastErr || tab.toast != "" {
			t.Fatal("success kept the old error")
		}
	}
}

func TestUsageLimitsFirstAndFooterPinned(t *testing.T) {
	for _, size := range [][2]int{{40, 16}, {80, 24}, {120, 34}, {200, 34}} {
		tab := tabWith(t, config{})
		tab.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		tab.Update(statusMsg{statuses: []Status{
			{AgentID: "claude-code", AuthLabel: "subscription", Limits: agent.RateStatus{Plan: "Max", Windows: []agent.RateWindow{
				{Label: "session 5h", UsedPercent: 42, ResetsAt: time.Now().Add(2 * time.Hour)},
				{Label: "week", UsedPercent: 91, ResetsAt: time.Now().Add(72 * time.Hour)}}}},
			{AgentID: "codex", AuthLabel: "subscription", Err: "no credentials"},
		}})
		view := tab.View()
		plain := ansi.Strip(view)
		lines := strings.Split(plain, "\n")
		if len(lines) != size[1] || lipgloss.Width(view) > size[0] {
			t.Fatalf("%v: screen %d rows / %d columns", size, len(lines), lipgloss.Width(view))
		}
		if !strings.Contains(lines[len(lines)-2]+lines[len(lines)-1], "help") && !strings.Contains(lines[len(lines)-3], "help") {
			t.Errorf("%v: hints are not pinned at the bottom:\n%s", size, plain)
		}
		limits, period, bar := strings.Index(plain, "Limits"), strings.Index(plain, "7 days"), strings.Index(plain, "42.0%")
		if limits < 0 || bar < 0 || (period >= 0 && period < limits) {
			t.Errorf("%v: limits should open the screen:\n%s", size, plain)
		}
		if strings.Contains(plain, "╭") || strings.Count(plain, "no credentials") != 1 {
			t.Errorf("%v: framed cards or repeated error:\n%s", size, plain)
		}
	}
}

// An agent without subscription limits (Pi) has no Status, yet its API-key
// cost must show: the cost column comes from every agent's auth mode.
func TestTabCostForAgentWithoutLimits(t *testing.T) {
	tab := newTab(New(nil, core.PathsIn(t.TempDir())), config{})
	tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	tab.Update(statusMsg{api: map[string]bool{"pi": true}})
	tab.Update(eventsMsg{events: []agent.UsageEvent{{AgentID: "pi", Time: time.Now().Add(-time.Hour), Model: "fake-model",
		Usage: agent.Usage{Input: 1000, Cost: 1.25}}}})
	if s := screen(&tab); !strings.Contains(s, "$1.25") {
		t.Fatalf("recorded cost missing:\n%s", s)
	}
}

// manyAgents is a tab with seven agents, each with limits (one with an API
// key and none) and a month of usage across models and projects.
func manyAgents(t *testing.T, w, h int) *Tab {
	t.Helper()
	now := time.Now()
	ids := []string{"claude-code", "codex", "gemini-cli", "opencode", "pi", "crush", "hermes"}
	models := []string{"claude-opus-5", "gpt-6", "gemini-3-pro", "qwen3-coder", "claude-sonnet-5", "kimi-k3", "glm-5"}
	projs := []string{"/home/u/workspace", "/home/u/api", "/home/u/shop", "/home/u/dashboard-with-a-long-name", "/home/u/infra"}
	var evs []agent.UsageEvent
	for d := 0; d < 30; d++ {
		for i, id := range ids {
			for k := 0; k < (d+i)%4+1; k++ {
				evs = append(evs, agent.UsageEvent{AgentID: id, Time: now.Add(-time.Duration(d)*24*time.Hour - time.Duration(k+i)*time.Hour),
					CWD: projs[(d+k+i)%len(projs)], Model: models[(i+k)%len(models)],
					Usage: agent.Usage{Input: 1000 * (i + 1), Output: 3000 * (k + 1), CacheRead: 200000 * (d%5 + 1)}, N: 3})
			}
		}
	}
	sort.Slice(evs, func(i, j int) bool { return evs[i].Time.Before(evs[j].Time) })
	var sts []Status
	for i, id := range ids {
		st := Status{AgentID: id, AuthLabel: "subscription", Limits: agent.RateStatus{Plan: []string{"max", "pro", "", "", "", "", ""}[i]}}
		st.Limits.Windows = []agent.RateWindow{{Label: "session 5h", UsedPercent: float64(10 * i), ResetsAt: now.Add(time.Duration(i+1) * time.Hour)},
			{Label: "week", UsedPercent: float64(12*i + 5), ResetsAt: now.Add(80 * time.Hour)}}
		if i == 0 {
			st.Limits.Windows = append(st.Limits.Windows, agent.RateWindow{Label: "week · Fable", UsedPercent: 3, ResetsAt: now.Add(80 * time.Hour)})
		}
		if i == 4 {
			st.Auth, st.AuthLabel = agent.AuthAPIKey, "API key"
			st.Limits.Windows = nil
		}
		sts = append(sts, st)
	}
	tab := newTab(New(nil, core.PathsIn(t.TempDir())), config{})
	tab.Update(tea.WindowSizeMsg{Width: w, Height: h})
	tab.Update(statusMsg{statuses: sts})
	tab.Update(eventsMsg{events: evs})
	return &tab
}

// Many agents never make one tall column: limit blocks sit side by side as
// far as the width allows, with bars aligned across them; on very wide
// terminals the other views' top rows sit beside the table.
func TestUsageManyAgentsUseTheWidth(t *testing.T) {
	for _, tc := range []struct{ w, perRow int }{{80, 1}, {120, 2}, {200, 3}, {290, 4}} {
		tab := manyAgents(t, tc.w, 80)
		plain := screen(tab)
		lines := strings.Split(plain, "\n")
		heads := 0
		for _, ln := range lines {
			if strings.Contains(ln, "● claude-code") {
				heads = strings.Count(ln, "● ")
			}
		}
		if heads != tc.perRow {
			t.Errorf("width %d: %d limit blocks on the first row, want %d:\n%s", tc.w, heads, tc.perRow, plain)
		}
		pcts := map[int]bool{}
		for _, ln := range lines {
			if i := strings.Index(ln, "% "); i >= 0 && strings.Contains(ln, "session 5h") {
				pcts[lipgloss.Width(ln[:i])] = true
			}
		}
		if tc.perRow == 1 && len(pcts) != 1 {
			t.Errorf("width %d: limit bars not aligned: %v", tc.w, pcts)
		}
		if tops := strings.Contains(plain, "Top models"); tops != (tc.w >= topsWidth) {
			t.Errorf("width %d: top rows shown = %v", tc.w, tops)
		}
		for _, ln := range lines {
			if lipgloss.Width(ln) > tc.w {
				t.Fatalf("width %d: line overflows: %q", tc.w, ln)
			}
		}
	}
}
