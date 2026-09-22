package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// run entrega o resultado de um tea.Cmd ao Update, encadeando até acabar.
func run(t *testing.T, m *Tab, cmd tea.Cmd) {
	t.Helper()
	for i := 0; cmd != nil && i < 5; i++ {
		cmd = m.Update(cmd())
	}
}

// TestInstallFlow cobre o caminho perigoso da aba: tecla → confirm → escrita
// no arquivo vivo do agente.
func TestInstallFlow(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	if err := svc.Save(Hook{Name: "doctor", Description: "roda o doctor",
		Hook: agent.Hook{Event: agent.HookSessionStart, Command: "lazyagents doctor"}}); err != nil {
		t.Fatal(err)
	}

	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	if m.Count() != 1 || len(m.statuses) != 1 {
		t.Fatalf("carga = %d hooks, %d agentes", m.Count(), len(m.statuses))
	}

	// 1 arma o confirm; nada é escrito antes do "sim".
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if !m.Capturing() {
		t.Fatal("tecla 1 deveria abrir o confirm")
	}
	if _, err := os.Stat(claude.HooksFile()); !os.IsNotExist(err) {
		t.Fatal("escreveu com o confirm ainda aberto")
	}
	if view := m.View(); !strings.Contains(view, "lazyagents doctor") || !strings.Contains(view, claude.HooksFile()) {
		t.Errorf("o confirm não diz o que vai mudar:\n%s", view)
	}

	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	data, err := os.ReadFile(claude.HooksFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "lazyagents doctor") {
		t.Errorf("hook não foi instalado:\n%s", data)
	}
	if len(m.statuses[0].Enabled) != 1 {
		t.Errorf("a aba não recarregou: %+v", m.statuses[0])
	}
	if view := m.View(); !strings.Contains(view, "●") {
		t.Errorf("matriz sem marcador:\n%s", view)
	}

	// 1 de novo remove.
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if len(m.statuses[0].Enabled) != 0 {
		t.Errorf("segundo 1 deveria remover: %+v", m.statuses[0])
	}
}

func TestEscCancelsWrite(t *testing.T) {
	svc, home := testService(t)
	claude := agent.NewClaude(home)
	if err := svc.Save(Hook{Name: "x", Hook: agent.Hook{Event: agent.HookStop, Command: "echo x"}}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Capturing() {
		t.Error("esc deveria fechar o confirm")
	}
	if _, err := os.Stat(claude.HooksFile()); !os.IsNotExist(err) {
		t.Error("esc escreveu no arquivo do agente")
	}
}

// A matriz marca com – o agente que não dispara o evento do hook.
func TestMatrixMarksUnsupportedEvent(t *testing.T) {
	svc, home := testService(t)
	svc.adapters = append(svc.adapters, agent.NewCodex(home))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(Hook{Name: "parada", Hook: agent.Hook{Event: agent.HookStop, Command: "echo x"}}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	if !strings.Contains(m.View(), "–") {
		t.Errorf("faltou o marcador de evento não suportado:\n%s", m.View())
	}
}
