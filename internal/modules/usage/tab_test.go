package usage

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// tabWith monta a aba já carregada com eventos de dois agentes e projetos.
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
	tab.Update(statusMsg{statuses: []Status{{AgentID: "claude-code", AuthLabel: "assinatura"}, {AgentID: "codex", AuthLabel: "assinatura"}}})
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
	if s := screen(tab); !strings.Contains(s, "Tokens por dia · 7 dias") || !strings.Contains(s, "110 tokens") {
		t.Fatalf("padrão deveria ser 7 dias por dia, sem o evento de 20 dias atrás:\n%s", s)
	}
	key(tab, "p") // 30 dias: entra o antigo
	if s := screen(tab); !strings.Contains(s, "1.1k tokens") || !strings.Contains(s, "5 respostas") {
		t.Errorf("30 dias deveria somar tudo (N conta respostas):\n%s", s)
	}
	key(tab, "right")
	key(tab, "right") // projeto
	key(tab, "a")     // só claude-code
	s := screen(tab)
	if !strings.Contains(s, "Tokens por projeto · 30 dias · claude-code") || !strings.Contains(s, "antigo") || strings.Contains(s, "beta") {
		t.Errorf("projeto + claude-code:\n%s", s)
	}
	key(tab, "esc") // sem texto: volta a todos os agentes
	if s := screen(tab); !strings.Contains(s, "beta") {
		t.Errorf("esc deveria voltar a todos os agentes:\n%s", s)
	}
}

func TestTabTextFilter(t *testing.T) {
	tab := tabWith(t, config{Period: "30d", View: "projects"})
	key(tab, "/")
	if !tab.Capturing() {
		t.Fatal("/ deveria abrir o input")
	}
	for _, r := range "ALP" {
		key(tab, string(r))
	}
	s := screen(tab)
	if !strings.Contains(s, "alpha") || strings.Contains(s, "beta") || strings.Contains(s, "antigo") {
		t.Errorf("filtro por texto (sem diferenciar maiúsculas):\n%s", s)
	}
	key(tab, "enter")
	if tab.Capturing() || tab.f.text != "ALP" {
		t.Errorf("enter deveria fechar mantendo o filtro: %+v", tab.f)
	}
	key(tab, "/")
	key(tab, "z")
	if s := screen(tab); !strings.Contains(s, `Nenhuma linha contém "ALPz"`) {
		t.Errorf("filtro sem resultado:\n%s", s)
	}
	key(tab, "esc")
	if tab.Capturing() || tab.f.text != "" {
		t.Errorf("esc no input deveria limpar: %+v", tab.f)
	}
}

func TestTabConfigAndPalette(t *testing.T) {
	tab := tabWith(t, config{Period: "all", View: "models"})
	if s := screen(tab); !strings.Contains(s, "Tokens por modelo · tudo") {
		t.Errorf("config period/view não aplicada:\n%s", s)
	}
	if f := newFilters(config{Period: "xyz", View: "nada"}); f.period != 1 || f.view != 0 {
		t.Errorf("valor desconhecido deveria cair no padrão: %+v", f)
	}
	var viewAgents tea.Msg
	for _, c := range tab.Commands() {
		if c.Name == "view agents" {
			viewAgents = c.Msg
		}
	}
	tab.Update(viewAgents)
	if s := screen(tab); !strings.Contains(s, "Tokens por agente") {
		t.Errorf("paleta view agents:\n%s", s)
	}
}

// A rolagem para no fim do conteúdo e o corpo só é refeito quando muda.
func TestTabScrollClampAndMemo(t *testing.T) {
	tab := tabWith(t, config{})
	for range 500 {
		key(tab, "j")
	}
	if max := max(0, len(tab.lines)-tab.contentHeight()); tab.scroll != max {
		t.Errorf("scroll = %d, quer no máximo %d", tab.scroll, max)
	}
	before := tab.drawn
	key(tab, "k")
	if tab.drawn != before {
		t.Error("rolar não deveria refazer o corpo")
	}
}

func TestFiltersStayVisibleWhileScrolling(t *testing.T) {
	tab := tabWith(t, config{Period: "30d", View: "projects"})
	tab.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
	for range 100 {
		key(tab, "j")
	}
	view := screen(tab)
	if !strings.Contains(view, "30 dias") || !strings.Contains(view, "projeto") || !strings.Contains(view, "atalhos") || lipgloss.Height(tab.View()) > 11 {
		t.Fatalf("filtros/ações fora da tela:\n%s", view)
	}
	before := tab.drawn
	key(tab, "k")
	if tab.drawn != before {
		t.Fatal("rolagem refez agregação")
	}
	key(tab, "/")
	tab.Update(tea.PasteMsg{Content: strings.Repeat("x", 50) + "FIM"})
	view = screen(tab)
	if !strings.Contains(view, "FIM") || !strings.Contains(view, "esc limpa") || lipgloss.Height(tab.View()) > 11 {
		t.Fatalf("input cortado:\n%s", view)
	}
}

func TestUsageLongErrorsAndCachedLimitsRemainReadable(t *testing.T) {
	for _, width := range []int{36, 76, 116} {
		tab := tabWith(t, config{})
		tab.Update(tea.WindowSizeMsg{Width: width, Height: 11})
		status := Status{AgentID: "codex", AuthLabel: "assinatura", Cached: true, Err: strings.Repeat("falha ao consultar serviço ", 20) + "DETALHE FINAL", Limits: agent.RateStatus{FetchedAt: time.Now().Add(-time.Hour), Windows: []agent.RateWindow{{Label: "Janela semanal por modelo", UsedPercent: 93.4, ResetsAt: time.Now().Add(time.Hour)}}}}
		tab.Update(statusMsg{statuses: []Status{status}})
		var seen strings.Builder
		for range len(tab.lines) + 2 {
			view := tab.View()
			if lipgloss.Width(view) > width || lipgloss.Height(view) > 11 {
				t.Fatalf("aviso fora da tela:\n%s", ansi.Strip(view))
			}
			seen.WriteString(ansi.Strip(view))
			seen.WriteByte('\n')
			key(tab, "j")
		}
		all := seen.String()
		for _, want := range []string{"DETALHE FINAL", "anteriores", "93.4%", "reseta", "r tenta novamente"} {
			if !strings.Contains(all, want) {
				t.Errorf("informação inacessível em %d colunas: %s", width, want)
			}
		}
		tab.Update(statusMsg{statuses: []Status{{AgentID: "codex", Err: "SEM CACHE FINAL"}}})
		key(tab, "home")
		if !strings.Contains(ansi.Strip(tab.body()), "SEM CACHE FINAL") {
			t.Fatal("erro sem cache foi cortado")
		}
	}
}

func TestUsageProgressWaitsForBothLoads(t *testing.T) {
	for _, limitsFirst := range []bool{false, true} {
		tab := tabWith(t, config{})
		tab.loadCmd(false) // apenas prepara os comandos, sem I/O
		status := statusMsg{statuses: []Status{{AgentID: "codex", Err: "falhou"}}}
		if limitsFirst {
			tab.Update(status)
		} else {
			tab.Update(eventsMsg{})
		}
		if !tab.loading || !strings.Contains(screen(tab), "atualizando uso") {
			t.Fatal("primeira resposta encerrou o progresso")
		}
		if limitsFirst {
			tab.Update(eventsMsg{})
		} else {
			tab.Update(status)
		}
		if tab.loading || !tab.toastErr || !strings.Contains(tab.toast, "1 aviso") {
			t.Fatal("fim da consulta perdeu o aviso")
		}
		tab.Update(statusMsg{})
		if tab.toastErr || tab.toast != "" {
			t.Fatal("sucesso manteve erro antigo")
		}
	}
}
