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
			Active: true, Profile: "trabalho", Applied: agent.ProviderProfile{BaseURL: "https://gw.example/v1"}},
		{AgentID: "codex", AgentName: "Codex", Short: "X", Installed: true, File: "/tmp/config.toml"},
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestProvidersInUseFirst(t *testing.T) {
	m := matrixTab(t, []agent.ProviderProfile{{Name: "local"}, {Name: "trabalho", BaseURL: "https://gw.example/v1"}}, 100, 24)
	plain := ansi.Strip(m.View())
	inUse, table := strings.Index(plain, "EM USO"), strings.Index(plain, "PERFIS")
	if inUse < 0 || table < 0 || inUse > table {
		t.Fatalf("\"em uso\" deveria vir antes dos perfis:\n%s", plain)
	}
	if !strings.Contains(plain, "usa o perfil trabalho · gw.example") || !strings.Contains(plain, "padrão do agente") {
		t.Errorf("estado atual dos agentes ausente:\n%s", plain)
	}
}

func TestProvidersEmptyStateSaysItOnce(t *testing.T) {
	for _, w := range []int{40, 100, 130} {
		m := matrixTab(t, nil, w, 24)
		plain := ansi.Strip(m.View())
		if strings.Count(plain, "Nenhum perfil") != 1 || !strings.Contains(plain, "EM USO") || lipgloss.Height(m.View()) > 24 {
			t.Errorf("%d: estado vazio repetido ou sem \"em uso\":\n%s", w, plain)
		}
	}
}

func TestProvidersSpaceTargetsAgentUnderCursor(t *testing.T) {
	m := matrixTab(t, []agent.ProviderProfile{{Name: "local", BaseURL: "http://localhost:4000"}}, 100, 24)
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "Aplicar local em Codex?") {
		t.Fatalf("space deveria perguntar pelo Codex:\n%s", ansi.Strip(m.View()))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(m.View(), "\x1b[7m") {
		t.Error("célula sob o cursor não aparece invertida")
	}
	m.Update(tea.KeyPressMsg{Code: 'a'})
	if m.confirm == nil || !strings.Contains(ansi.Strip(m.View()), "todos os agentes") {
		t.Error("a deveria aplicar em todos")
	}
}

func TestProvidersClickCell(t *testing.T) {
	m := matrixTab(t, []agent.ProviderProfile{{Name: "local"}, {Name: "trabalho"}}, 100, 24)
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
		t.Fatalf("clique na célula: perfil %d agente %d", m.cursor, m.col)
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 1}) // "em uso": só leitura
	if m.cursor != 1 {
		t.Error("clique no \"em uso\" mudou o perfil")
	}
}
