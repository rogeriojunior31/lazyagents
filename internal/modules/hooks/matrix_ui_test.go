package hooks

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

func matrixTab(t *testing.T, w, h int) *Tab {
	t.Helper()
	svc, _ := testService(t)
	tab := newTab(svc)
	m := &tab
	m.lib = []Hook{
		{Name: "formatar", Hooks: []agent.Hook{{Event: agent.HookPostToolUse, Command: "echo fmt"}}},
		{Name: "pacote", Off: []int{1}, Hooks: []agent.Hook{
			{Event: agent.HookSessionStart, Command: "echo a"}, {Event: agent.HookStop, Command: "echo b"}, {Event: agent.HookStop, Command: "echo c"}}},
	}
	m.statuses = []Status{
		{AgentID: "claude-code", AgentName: "Claude Code", Short: "C", Installed: true, File: "/tmp/settings.json",
			Events: []string{agent.HookPostToolUse, agent.HookSessionStart, agent.HookStop}, Enabled: []string{"formatar"}},
		{AgentID: "codex", AgentName: "Codex", Short: "X", Installed: true, File: "/tmp/hooks.json",
			Events: []string{agent.HookSessionStart, agent.HookStop}},
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestHooksMatrixRows(t *testing.T) {
	for _, size := range [][2]int{{40, 16}, {80, 24}, {130, 30}} {
		m := matrixTab(t, size[0], size[1])
		view := m.View()
		plain := ansi.Strip(view)
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("%v: fora da área", size)
		}
		for _, want := range []string{"LIBRARY", "formatar", "pacote", "2/3", "●", "–"} {
			if !strings.Contains(plain, want) {
				t.Errorf("%v: falta %q:\n%s", size, want, plain)
			}
		}
	}
}

func TestHooksSpaceTargetsAgentUnderCursor(t *testing.T) {
	m := matrixTab(t, 100, 24)
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "Uninstall hook formatar from Claude Code?") {
		t.Fatalf("space na coluna do Claude deveria remover de lá:\n%s", ansi.Strip(m.View()))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "Install hook pacote in Codex?") {
		t.Fatalf("space na coluna do Codex deveria instalar lá:\n%s", ansi.Strip(m.View()))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.cmdMode {
		t.Fatal("→ não deve mais abrir a seleção de comandos")
	}
	m.Update(tea.KeyPressMsg{Code: 'a'})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "in every agent") {
		t.Error("a deveria instalar em todos")
	}
}

func TestHooksClickCell(t *testing.T) {
	m := matrixTab(t, 100, 24)
	sp := m.split()
	cols := m.tableCols(sp.ListW)
	x := -1
	for i := range sp.ListW {
		if kit.ColumnAt(sp.ListW, cols, i) == colAgents+1 {
			x = i
			break
		}
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: hooksTop + 1})
	if m.cursor != 1 || m.col != 1 {
		t.Fatalf("clique na célula: hook %d agente %d", m.cursor, m.col)
	}
}
