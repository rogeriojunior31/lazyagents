package skills

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// matrixTab builds the tab with three agents and n skills. The first reaches
// Claude via another dir (◆) and is local in Codex (▪): toggling those never
// touches disk, so the error shows which agent the key targeted.
func matrixTab(t *testing.T, n, w, h int) Tab {
	t.Helper()
	m := newTab(New(core.PathsIn(t.TempDir())))
	m.targets = []agent.Agent{
		{ID: "claude-code", Name: "Claude Code", Short: "C"},
		{ID: "codex", Name: "Codex", Short: "X"},
		{ID: "gemini-cli", Name: "Gemini CLI", Short: "G"},
	}
	m.agents = m.targets
	for i := range n {
		sk := Skill{Dir: fmt.Sprintf("skill-%02d", i), Name: fmt.Sprintf("skill-%02d", i), InLibrary: true, Valid: true,
			Description: "description " + strings.Repeat("long ", 20) + "END", States: map[string]AgentState{}}
		if i == 0 {
			sk.States["claude-code"] = AgentState{On: true, Via: "/shared"}
			sk.States["codex"] = AgentState{On: true, Local: true}
		}
		m.skills = append(m.skills, sk)
	}
	m.rebuildListItems()
	m, _ = m.update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func toggleErr(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("space produced no action")
	}
	msg, ok := cmd().(skillOpMsg)
	if !ok || msg.err == nil {
		t.Fatalf("want a not-toggleable warning, got %#v", msg)
	}
	return msg.err.Error()
}

func TestMatrixSpaceTogglesAgentUnderCursor(t *testing.T) {
	m := matrixTab(t, 3, 100, 24)
	_, cmd := m.updateList(tea.KeyPressMsg{Code: tea.KeySpace})
	if err := toggleErr(t, cmd); !strings.Contains(err, "Claude Code") {
		t.Errorf("column 0 should target Claude: %s", err)
	}
	m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyRight})
	_, cmd = m.updateList(tea.KeyPressMsg{Code: tea.KeySpace})
	if err := toggleErr(t, cmd); !strings.Contains(err, "Codex") {
		t.Errorf("→ should target Codex: %s", err)
	}
	for range 5 {
		m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if m.col != 2 {
		t.Errorf("column cursor went past the last agent: %d", m.col)
	}
	if !strings.Contains(m.View(), "\x1b[7m") {
		t.Error("cell under the cursor not inverted on the selected row")
	}
}

func TestMatrixFitsAndKeepsSelectionVisible(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 20}, {130, 30}} {
		w, h := size[0], size[1]
		m := matrixTab(t, 25, w, h)
		for i := range 25 {
			view := m.View()
			plain := ansi.Strip(view)
			if lipgloss.Width(view) > w || lipgloss.Height(view) > h {
				t.Fatalf("%v: view %dx%d out of bounds", size, lipgloss.Width(view), lipgloss.Height(view))
			}
			if !strings.Contains(plain, fmt.Sprintf("skill-%02d", i)) || !strings.Contains(plain, "?") {
				t.Fatalf("%v: skill %d or help off screen:\n%s", size, i, plain)
			}
			m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyDown})
		}
	}
}

func TestMatrixDetailScrollsToEnd(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {130, 20}} {
		m := matrixTab(t, 25, size[0], size[1])
		for range 30 {
			m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		if !strings.Contains(ansi.Strip(m.View()), "library") {
			t.Errorf("%v: end of the detail (SOURCE) unreachable:\n%s", size, ansi.Strip(m.View()))
		}
		if m.list.Index() != 0 {
			t.Errorf("%v: shift+↓ changed the selected skill", size)
		}
	}
}

func TestMatrixMouse(t *testing.T) {
	m := matrixTab(t, 5, 100, 24)
	sp := m.split()
	cols := m.tableCols(sp.ListW)
	_, _, top := m.tableWindow(sp.ListW, sp.ListH)
	// x in the middle of the Gemini column (third agent)
	x := -1
	for i := range sp.ListW {
		if kit.ColumnAt(sp.ListW, cols, i) == colAgents+2 {
			x = i
			break
		}
	}
	m, _ = m.click(tea.MouseClickMsg{X: x, Y: top + 3, Button: tea.MouseLeft})
	if m.list.Index() != 3 || m.col != 2 {
		t.Fatalf("cell click: row %d column %d", m.list.Index(), m.col)
	}
	// the wheel over the detail scrolls it, not the skill
	m, _ = m.update(tea.MouseWheelMsg{X: 1, Y: sp.ListH + 1, Button: tea.MouseWheelDown})
	if m.list.Index() != 3 {
		t.Error("wheel over the detail changed the skill")
	}
	m, _ = m.update(tea.MouseWheelMsg{X: 1, Y: top, Button: tea.MouseWheelDown})
	if m.list.Index() != 4 {
		t.Error("wheel over the matrix did not move the cursor")
	}
}

// The detail's ▸ marks the agent space toggles; with no digit shortcut, it
// must follow ←/→ at once.
func TestDetailMarkerFollowsColumn(t *testing.T) {
	m := matrixTab(t, 1, 100, 40)
	m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyRight})
	detail := ansi.Strip(m.detailVP.View())
	if !strings.Contains(detail, "▸ ◆ Codex") && !strings.Contains(detail, "▸ ▪ Codex") {
		t.Errorf("▸ not on Codex after →:\n%s", detail)
	}
}

// The filter result arrives async (list.FilterMatchesMsg): the detail must
// follow the new selection, and the "Filter:" line must fit the table column
// or it pushes the detail past the screen edge.
func TestFilterKeepsDetailAndWidth(t *testing.T) {
	const w, h = 160, 30
	m := matrixTab(t, 4, w, h)
	// only the filter result is fed back: the cursor blink ticks forever
	feed := func(msg tea.Msg) {
		var cmd tea.Cmd
		m, cmd = m.update(msg)
		cmds := []tea.Cmd{cmd}
		for len(cmds) > 0 {
			c := cmds[0]
			cmds = cmds[1:]
			if c == nil {
				continue
			}
			done := make(chan tea.Msg, 1)
			go func() { done <- c() }()
			select {
			case out := <-done:
				switch out := out.(type) {
				case tea.BatchMsg:
					cmds = append(cmds, out...)
				case list.FilterMatchesMsg:
					m, cmd = m.update(out)
					cmds = append(cmds, cmd)
				}
			case <-time.After(50 * time.Millisecond): // a timer (blink): drop it
			}
		}
	}
	feed(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range "skill-02" {
		feed(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if sel, _ := m.selected(); sel.Name != "skill-02" {
		t.Fatalf("filter selected %q", sel.Name)
	}
	if !strings.Contains(ansi.Strip(m.detailVP.View()), "skill-02") {
		t.Fatalf("detail still shows the previous skill:\n%s", ansi.Strip(m.detailVP.View()))
	}
	sp := m.split()
	for i, l := range strings.Split(m.tableView(sp.ListW, sp.ListH), "\n") {
		if lipgloss.Width(l) > sp.ListW {
			t.Fatalf("table line %d is %d wide, column is %d: %q", i, lipgloss.Width(l), sp.ListW, ansi.Strip(l))
		}
	}
	for i, l := range strings.Split(m.View(), "\n") {
		if lipgloss.Width(l) > w {
			t.Fatalf("view line %d is %d wide, screen is %d", i, lipgloss.Width(l), w)
		}
	}
}
