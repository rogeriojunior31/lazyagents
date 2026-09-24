package sessions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// tableTab monta a aba com n conversas do agente "a", em três projetos.
func tableTab(t *testing.T, n, w, h int) *Tab {
	t.Helper()
	home := t.TempDir()
	tab := newTab(New([]agent.Adapter{fakeAdapter{id: "a"}}, core.PathsIn(home)), home)
	m := &tab
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	var ss []agent.Session
	for i := range n {
		ss = append(ss, agent.Session{ID: fmt.Sprintf("s%02d", i), AgentID: "a", AgentName: "Agente",
			Title: fmt.Sprintf("conversa-%02d %s", i, strings.Repeat("texto ", 15)), CWD: fmt.Sprintf("/p/proj%d", i%3),
			MTime: time.Now().Add(-time.Duration(i) * time.Hour)})
	}
	m.Update(events.SessionsLoaded{Sessions: ss})
	return m
}

func TestSessionTableOneLinePerConversation(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 20}, {130, 30}} {
		w, h := size[0], size[1]
		m := tableTab(t, 30, w, h)
		for i := range 30 {
			view := m.View()
			plain := ansi.Strip(view)
			if lipgloss.Width(view) > w || lipgloss.Height(view) > h {
				t.Fatalf("%v: view %dx%d fora da área", size, lipgloss.Width(view), lipgloss.Height(view))
			}
			if !strings.Contains(plain, fmt.Sprintf("conversa-%02d", i)) || !strings.Contains(plain, "atalhos") {
				t.Fatalf("%v: conversa %d ou ajuda fora da tela:\n%s", size, i, plain)
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
	}
	// Com espaço, cabem muito mais conversas que as ~4 do layout de 3 linhas.
	m := tableTab(t, 30, 130, 30)
	if got := strings.Count(ansi.Strip(m.View()), "conversa-"); got < 20 {
		t.Errorf("só %d conversas visíveis em 130×30", got)
	}
}

func TestSessionDetailShowsResumeFirst(t *testing.T) {
	m := tableTab(t, 3, 80, 20)
	plain := ansi.Strip(m.View())
	resume, agentLine := strings.Index(plain, "a resume s00"), strings.Index(plain, "agente  Agente")
	if resume < 0 {
		t.Fatalf("comando de retomar fora da faixa de detalhe:\n%s", plain)
	}
	if agentLine >= 0 && agentLine < resume {
		t.Error("metadados antes do comando de retomar")
	}
	// A pasta da sessão (/p/proj0) não existe: o adapter retoma em /dir/a e o
	// detalhe avisa.
	for range 10 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	}
	if !strings.Contains(ansi.Strip(m.View()), "não existe mais") && !strings.Contains(m.detailVP.View(), "não existe mais") {
		t.Error("sem aviso de pasta ausente")
	}
	if m.list.Index() != 0 {
		t.Error("shift+↓ mudou a conversa selecionada")
	}
}

func TestSessionGroupHeadersAndBatchSelect(t *testing.T) {
	m := tableTab(t, 6, 100, 24)
	m.Update(tea.KeyPressMsg{Code: 'g'})
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "▸ proj0 · a (2)") {
		t.Fatalf("cabeçalho de grupo ausente:\n%s", plain)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // no cabeçalho: seleciona o grupo
	if n := len(m.selectedSessions()); n != 2 {
		t.Fatalf("grupo selecionou %d conversas", n)
	}
	if plain = ansi.Strip(m.View()); !strings.Contains(plain, "2 selecionada(s)") || strings.Count(plain, "✓") != 2 {
		t.Errorf("seleção em lote não aparece:\n%s", plain)
	}
}

func TestSessionMouseByRegion(t *testing.T) {
	m := tableTab(t, 10, 80, 24)
	sp := m.split()
	_, _, top := m.tableWindow(sp.ListW, sp.ListH)
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: top + 2})
	if m.list.Index() != 2 {
		t.Fatalf("clique selecionou %d", m.list.Index())
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: sp.ListH + 1})
	if m.list.Index() != 2 {
		t.Error("roda sobre o detalhe trocou a conversa")
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: top})
	if m.list.Index() != 3 {
		t.Error("roda sobre a tabela não desceu")
	}
}
