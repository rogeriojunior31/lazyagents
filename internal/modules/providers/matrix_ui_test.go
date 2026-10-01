package providers

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

func matrixTab(t *testing.T, profiles []agent.ProviderProfile, w, h int) *Tab {
	t.Helper()
	tab := newTab(New(nil, core.PathsIn(t.TempDir())))
	m := &tab
	m.profiles = profiles
	m.statuses = []Status{
		{AgentID: "claude-code", AgentName: "Claude Code", Short: "C", Installed: true, File: "/tmp/claude.json",
			Active: true, Profile: "work", Applied: agent.ProviderProfile{BaseURL: "https://gw.example/v1"}},
		{AgentID: "codex", AgentName: "Codex", Short: "X", Installed: true, File: "/tmp/config.toml"},
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestProvidersInUseFirst(t *testing.T) {
	m := matrixTab(t, []agent.ProviderProfile{{Name: "local"}, {Name: "work", BaseURL: "https://gw.example/v1"}}, 100, 24)
	plain := ansi.Strip(m.View())
	inUse, table := strings.Index(plain, "IN USE"), strings.Index(plain, "PROFILES")
	if inUse < 0 || table < 0 || inUse > table {
		t.Fatalf("\"in use\" should come before profiles:\n%s", plain)
	}
	if !strings.Contains(plain, "uses profile work") || !strings.Contains(plain, "gw.example") || !strings.Contains(plain, "agent default") {
		t.Errorf("current agent state missing:\n%s", plain)
	}
	if narrow := ansi.Strip(matrixTab(t, nil, 80, 24).View()); !strings.Contains(narrow, "uses profile work · gw.example") {
		t.Errorf("narrow: the host should follow the state:\n%s", narrow)
	}
}

// "In use" carries each agent's file, so with no profile there is no detail
// panel repeating it; wide, the table spans the whole width.
func TestProvidersInUseHasFilesAndNoEmptyDetail(t *testing.T) {
	m := matrixTab(t, nil, 200, 24)
	plain := ansi.Strip(m.View())
	inUse := plain[:strings.Index(plain, "PROFILES")]
	if !strings.Contains(inUse, "/tmp/claude.json") || !strings.Contains(inUse, "/tmp/config.toml") {
		t.Errorf("files missing from \"in use\":\n%s", plain)
	}
	if strings.Contains(strings.ReplaceAll(plain, "PROFILES", ""), "FILES") || strings.Count(plain, "/tmp/claude.json") != 1 {
		t.Errorf("an empty library still shows a detail panel:\n%s", plain)
	}
}

func TestProvidersEmptyStateSaysItOnce(t *testing.T) {
	for _, w := range []int{40, 100, 130} {
		m := matrixTab(t, nil, w, 24)
		plain := ansi.Strip(m.View())
		if strings.Count(plain, "No profiles") != 1 || !strings.Contains(plain, "IN USE") || lipgloss.Height(m.View()) > 24 {
			t.Errorf("%d: empty state repeated or without \"in use\":\n%s", w, plain)
		}
	}
}

func TestProvidersSpaceTargetsAgentUnderCursor(t *testing.T) {
	m := matrixTab(t, []agent.ProviderProfile{{Name: "local", BaseURL: "http://localhost:4000"}}, 100, 24)
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "Apply local to Codex?") {
		t.Fatalf("space should ask about Codex:\n%s", ansi.Strip(m.View()))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(m.View(), "\x1b[7m") {
		t.Error("cell under the cursor is not inverted")
	}
	m.Update(tea.KeyPressMsg{Code: 'a'})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "every installed agent") {
		t.Error("a should apply to all")
	}
}

func TestProvidersClickCell(t *testing.T) {
	m := matrixTab(t, []agent.ProviderProfile{{Name: "local"}, {Name: "work"}}, 100, 24)
	sp := m.split()
	cols := m.tableCols(sp.ListW)
	x := -1
	for i := range sp.ListW {
		if kit.ColumnAt(sp.ListW, cols, i) == colAgents+1 {
			x = i
			break
		}
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: m.inUseHeight() + profilesTop + 1})
	if m.cursor != 1 || m.col != 1 {
		t.Fatalf("cell click: profile %d agent %d", m.cursor, m.col)
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 1}) // "in use" is read-only
	if m.cursor != 1 {
		t.Error("click on \"in use\" changed the profile")
	}
}
