package skills

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// matrixTab monta a aba com três agentes e n skills: a primeira chega ao
// Claude por outro diretório (◆) e é local no Codex (▪) — estados que não
// tocam o disco ao alternar, então dá para ver qual agente a tecla mirou.
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
			Description: "descrição " + strings.Repeat("longa ", 20) + "FIM", States: map[string]AgentState{}}
		if i == 0 {
			sk.States["claude-code"] = AgentState{On: true, Via: "/compartilhado"}
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
		t.Fatal("space não gerou ação")
	}
	msg, ok := cmd().(skillOpMsg)
	if !ok || msg.err == nil {
		t.Fatalf("esperava aviso de estado não alternável, veio %#v", msg)
	}
	return msg.err.Error()
}

func TestMatrixSpaceTogglesAgentUnderCursor(t *testing.T) {
	m := matrixTab(t, 3, 100, 24)
	_, cmd := m.updateList(tea.KeyPressMsg{Code: tea.KeySpace})
	if err := toggleErr(t, cmd); !strings.Contains(err, "Claude Code") {
		t.Errorf("coluna 0 deveria mirar o Claude: %s", err)
	}
	m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyRight})
	_, cmd = m.updateList(tea.KeyPressMsg{Code: tea.KeySpace})
	if err := toggleErr(t, cmd); !strings.Contains(err, "Codex") {
		t.Errorf("→ deveria mirar o Codex: %s", err)
	}
	for range 5 {
		m, _ = m.updateList(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if m.col != 2 {
		t.Errorf("cursor de coluna passou do último agente: %d", m.col)
	}
	if !strings.Contains(m.View(), "\x1b[7m") {
		t.Error("célula sob o cursor não aparece invertida na linha selecionada")
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
				t.Fatalf("%v: view %dx%d fora da área", size, lipgloss.Width(view), lipgloss.Height(view))
			}
			if !strings.Contains(plain, fmt.Sprintf("skill-%02d", i)) || !strings.Contains(plain, "?") {
				t.Fatalf("%v: skill %d ou ajuda fora da tela:\n%s", size, i, plain)
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
			t.Errorf("%v: fim do detalhe (ORIGEM) inalcançável:\n%s", size, ansi.Strip(m.View()))
		}
		if m.list.Index() != 0 {
			t.Errorf("%v: shift+↓ mudou a skill selecionada", size)
		}
	}
}

func TestMatrixMouse(t *testing.T) {
	m := matrixTab(t, 5, 100, 24)
	sp := m.split()
	cols := m.tableCols(sp.ListW)
	_, _, top := m.tableWindow(sp.ListW, sp.ListH)
	// x do meio da coluna do Gemini (terceiro agente)
	x := -1
	for i := range sp.ListW {
		if kit.ColumnAt(sp.ListW, cols, i) == colAgents+2 {
			x = i
			break
		}
	}
	m, _ = m.click(tea.MouseClickMsg{X: x, Y: top + 3, Button: tea.MouseLeft})
	if m.list.Index() != 3 || m.col != 2 {
		t.Fatalf("clique na célula: linha %d coluna %d", m.list.Index(), m.col)
	}
	// roda sobre o detalhe rola o texto, não troca de skill
	m, _ = m.update(tea.MouseWheelMsg{X: 1, Y: sp.ListH + 1, Button: tea.MouseWheelDown})
	if m.list.Index() != 3 {
		t.Error("roda sobre o detalhe trocou a skill")
	}
	m, _ = m.update(tea.MouseWheelMsg{X: 1, Y: top, Button: tea.MouseWheelDown})
	if m.list.Index() != 4 {
		t.Error("roda sobre a matriz não desceu o cursor")
	}
}
