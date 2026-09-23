package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// run entrega o resultado de um tea.Cmd ao Update, encadeando até acabar.
func run(t *testing.T, m *Tab, cmd tea.Cmd) {
	t.Helper()
	for i := 0; cmd != nil && i < 5; i++ {
		msg := cmd()
		cmd = m.Update(msg)
	}
}

// TestApplyFlow cobre o caminho perigoso da aba: tecla → confirm → escrita no
// arquivo vivo do agente.
func TestApplyFlow(t *testing.T) {
	home := t.TempDir()
	claude := agent.NewClaude(home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil { // faz o Detect ver instalado
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{claude}, core.PathsIn(home))
	if err := svc.Save(agent.ProviderProfile{Name: "nuvem", BaseURL: "https://nuvem/v1", Token: "segredo"}); err != nil {
		t.Fatal(err)
	}

	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	if m.Count() != 1 || len(m.statuses) != 1 {
		t.Fatalf("carga = %d perfis, %d agentes", m.Count(), len(m.statuses))
	}
	// A aba só aplica por nome: o token nem chega ao model.
	if p := m.profiles[0]; p.Token != "" || !p.HasToken {
		t.Errorf("perfil na aba = %+v", p)
	}

	// 1 arma o confirm; nada é escrito antes do "sim".
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if !m.Capturing() {
		t.Fatal("tecla 1 deveria abrir o confirm")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("o confirm ainda estava aberto e o arquivo já foi escrito")
	}
	if view := m.View(); !strings.Contains(view, "nuvem") || !strings.Contains(view, "~/.claude/settings.json") {
		t.Errorf("o confirm não diz o que vai mudar:\n%s", view)
	}

	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.Capturing() {
		t.Error("confirm continuou aberto depois do sim")
	}
	data, err := os.ReadFile(claude.ProviderFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://nuvem/v1") {
		t.Errorf("perfil não foi aplicado:\n%s", data)
	}
	if m.statuses[0].Profile != "nuvem" {
		t.Errorf("a aba não recarregou o estado: %+v", m.statuses[0])
	}
	if view := m.View(); !strings.Contains(view, "●") || strings.Contains(view, "segredo") {
		t.Errorf("view errada (ou com token):\n%s", view)
	}

	// 1 de novo no agente que já tem o perfil = remover.
	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.statuses[0].Active {
		t.Errorf("segundo 1 deveria limpar: %+v", m.statuses[0])
	}
}

func TestCancelDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	claude := agent.NewClaude(home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{claude}, core.PathsIn(home))
	if err := svc.Save(agent.ProviderProfile{Name: "nuvem", BaseURL: "https://nuvem/v1"}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))

	m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd != nil {
		t.Error("esc não deveria devolver comando")
	}
	if m.Capturing() {
		t.Error("esc deveria fechar o confirm")
	}
	if _, err := os.Stat(claude.ProviderFile()); !os.IsNotExist(err) {
		t.Error("esc escreveu no arquivo do agente")
	}
}

// typeText digita s no campo focado do formulário.
func typeText(m *Tab, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// O formulário cria o perfil; na edição, token vazio mantém o salvo e o
// valor nunca aparece na tela.
func TestProfileForm(t *testing.T) {
	home := t.TempDir()
	svc := New(nil, core.PathsIn(home))
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if !m.Capturing() {
		t.Fatal("n deveria abrir o formulário")
	}
	typeText(&m, "nuvem")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	typeText(&m, "ftp://x") // inválida: o formulário continua aberto com o erro
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.form == nil || !strings.Contains(m.View(), "http") {
		t.Fatalf("URL inválida deveria manter o formulário com erro:\n%s", m.View())
	}
	for range 7 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typeText(&m, "https://nuvem/v1")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	typeText(&m, "segredo")
	if strings.Contains(m.View(), "segredo") {
		t.Fatal("token digitado aparece em claro")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.form != nil {
		t.Fatalf("formulário não fechou: %s", m.form.err)
	}
	p, err := svc.Profile("nuvem")
	if err != nil || p.BaseURL != "https://nuvem/v1" || p.Token != "segredo" {
		t.Fatalf("perfil salvo = %+v, %v", p, err)
	}

	// Editar: troca o modelo e renomeia, sem digitar token.
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	typeText(&m, "2")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	typeText(&m, "big")
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if _, err := svc.Profile("nuvem"); err == nil {
		t.Error("renomear deveria tirar o nome antigo")
	}
	p, err = svc.Profile("nuvem2")
	if err != nil || p.Model != "big" || p.Token != "segredo" {
		t.Fatalf("edição = %+v, %v", p, err)
	}
	for _, prof := range m.profiles {
		if prof.Token != "" {
			t.Errorf("token chegou à aba: %+v", prof)
		}
	}
}

// Clique na lista seleciona o perfil da linha.
func TestClickSelectsProfile(t *testing.T) {
	svc := New(nil, core.PathsIn(t.TempDir()))
	for _, n := range []string{"a", "b"} {
		if err := svc.Save(agent.ProviderProfile{Name: n, Model: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Init())
	m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft}) // 2º item
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m.Update(tea.MouseClickMsg{X: 80, Y: 2, Button: tea.MouseLeft}) // detalhe: ignora
	if m.cursor != 1 {
		t.Errorf("clique no detalhe mexeu no cursor: %d", m.cursor)
	}
}
