package providers

import (
	"fmt"
	"net/url"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// bodyHeight é a altura do corpo, descontados hints e toast.
func (m Tab) bodyHeight() int { return max(6, m.height-2) }

// inUseHeight é o bloco "EM USO": título, um agente por linha e um respiro.
func (m Tab) inUseHeight() int { return min(2+len(m.statuses), max(0, m.bodyHeight()-4)) }

// split reparte o que sobra abaixo do "EM USO" entre a tabela de perfis e o
// detalhe; empilhado, a altura que a tabela não usa vai para o detalhe.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight()-m.inUseHeight())
	if !sp.Side {
		need := profilesTop + max(1, len(m.profiles)) + 1
		if spare := sp.ListH - need; spare > 0 {
			sp.ListH -= spare
			sp.DetailH += spare
		}
	}
	return sp
}

// View limita tudo à largura da aba: rede de segurança para terminal estreito.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if m.confirm != nil {
		return m.confirm.ViewIn(m.width, m.height)
	}
	if m.form != nil {
		return m.form.view(m.width, m.height)
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  lendo perfis e configs…")
	}
	if len(m.statuses) == 0 {
		return kit.StHint.Render("  nenhum agente instalado suporta troca de provedor.")
	}

	sp := m.split()
	table := m.profilesView(sp.ListW, sp.ListH)
	detail := kit.DetailView(sp, m.detailTitle(), m.detailViewport(sp))
	body := lipgloss.JoinVertical(lipgloss.Left, table, detail)
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", detail)
	}
	hints := kit.Hints(m.width, [2]string{"n", "novo perfil"}, [2]string{"x", "volta ao padrão"}, [2]string{"?", "atalhos"})
	if len(m.profiles) > 0 {
		hints = kit.Hints(m.width, [2]string{"space", "aplica/remove"}, [2]string{"←→", "agente"},
			[2]string{"a", "todos"}, [2]string{"n", "novo"}, [2]string{"e", "editar"},
			[2]string{"x", "volta ao padrão"}, [2]string{"?", "atalhos"})
	}
	out := []string{kit.Frame(m.inUseView(m.width), "", m.inUseHeight()), body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// inUseView é o que cada agente usa agora — a pergunta que traz à aba.
func (m Tab) inUseView(w int) string {
	nameW := 6
	for _, st := range m.statuses {
		nameW = max(nameW, 2+lipgloss.Width(st.AgentName))
	}
	cols := []kit.Column{{Width: nameW}, {Flex: true}}
	lines := []string{"  " + kit.StTitle.Render("EM USO")}
	for _, st := range m.statuses {
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render("● " + st.AgentName)
		mark, label := agentState(st, agent.ProviderProfile{}, false)
		if st.Active {
			label += kit.StHint.Render(" · " + endpointHost(st.Applied.BaseURL))
		}
		lines = append(lines, kit.TableRow(w, false, cols, name, mark+" "+label))
	}
	return strings.Join(lines, "\n")
}

// profilesTop é o título mais a linha de cabeçalho da tabela de perfis.
const profilesTop = 2

// Colunas da tabela de perfis; os agentes vêm a partir de colAgents.
const (
	colName = iota
	colHost
	colModel
	colAgents
)

// statusAgents converte os agentes da aba no formato das colunas de agente.
func (m Tab) statusAgents() []agent.Agent {
	ags := make([]agent.Agent, len(m.statuses))
	for i, st := range m.statuses {
		ags[i] = agent.Agent{ID: st.AgentID, Name: st.AgentName, Short: st.Short}
	}
	return ags
}

// tableCols: perfil, host do endpoint (flex), modelo e um agente por coluna,
// com a coluna do cursor sublinhada.
func (m Tab) tableCols(width int) []kit.Column {
	agents := kit.AgentColumns(m.statusAgents(), width/3)
	for i, st := range m.statuses {
		if i == m.col {
			agents[i].Title = lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Underline(true).Bold(true).
				Render(ansi.Strip(agents[i].Title))
		}
	}
	nameW, modelW := 6, 6
	for _, p := range m.profiles {
		nameW = max(nameW, lipgloss.Width(p.Name))
		modelW = max(modelW, lipgloss.Width(p.Model))
	}
	cols := []kit.Column{
		{Title: "perfil", Width: min(nameW, 20)},
		{Title: "endpoint", Flex: true},
		{Title: "modelo", Width: min(modelW, 20)},
	}
	if width < 64 {
		cols[colModel] = kit.Column{}
	}
	return append(cols, agents...)
}

// cells é a linha de um perfil: ● onde está aplicado. Na linha selecionada a
// célula do agente sob o cursor aparece invertida — é a que space alterna.
func (m Tab) cells(p agent.ProviderProfile, selected bool) []string {
	out := []string{p.Name, kit.StHint.Render(endpointHost(p.BaseURL)), kit.StHint.Render(p.Model)}
	for i, st := range m.statuses {
		mark := kit.StOff.Render("○")
		switch {
		case !st.Installed:
			mark = kit.StOff.Render("–")
		case st.Profile == p.Name:
			mark = kit.StOn.Render("●")
		}
		if selected && i == m.col {
			mark = lipgloss.NewStyle().Reverse(true).Render(ansi.Strip(mark))
		}
		out = append(out, mark)
	}
	return out
}

func (m Tab) profilesView(w, h int) string {
	title := kit.StTitle.Render("PERFIS") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.profiles)))
	cols := m.tableCols(w)
	lines := []string{"  " + title, kit.TableHeader(w, cols)}
	if len(m.profiles) == 0 {
		lines = append(lines[:1],
			kit.StHint.Render("  Nenhum perfil ainda — ")+components.Keycap("n")+kit.StHint.Render(" cria um aqui, ou pela CLI:"),
			kit.CardValue.Render("  lazyagents provider add trabalho --base-url https://… --token -"))
	}
	start, end := kit.Window(m.cursor, len(m.profiles), max(1, h-profilesTop))
	for i := start; i < end; i++ {
		lines = append(lines, kit.TableRow(w, i == m.cursor, cols, m.cells(m.profiles[i], i == m.cursor)...))
	}
	return kit.Frame(strings.Join(lines, "\n"), "", h)
}

// endpointHost encurta a URL para o host, que é o que distingue um provedor
// de outro numa linha estreita.
func endpointHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func (m Tab) detailTitle() string {
	if p, ok := m.current(); ok {
		return strings.ToUpper(p.Name)
	}
	return "ARQUIVOS"
}

// detailViewport monta o viewport do detalhe na posição de leitura atual.
func (m Tab) detailViewport(sp kit.Split) viewport.Model {
	w, h := kit.DetailSize(sp)
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(ansi.Wrap(m.detailContent(w), w, ""))
	vp.SetYOffset(m.detailOff)
	return vp
}

func (m *Tab) scrollDetail(msg tea.Msg) {
	vp, _ := m.detailViewport(m.split()).Update(msg)
	m.detailOff = vp.YOffset()
}

// detailContent é o perfil inteiro (endpoint completo, modelo, token) e,
// por agente, o arquivo que aplicar reescreve e o que está nele hoje.
func (m Tab) detailContent(inner int) string {
	var b strings.Builder
	pr, ok := m.current()
	if ok {
		b.WriteString(field("endpoint", orDash(pr.BaseURL), inner))
		b.WriteString(field("modelo", orDefault(pr.Model, "padrão do agente"), inner))
		b.WriteString(field("token", tokenLabel(pr), inner))
		if pr.WireAPI != "" {
			b.WriteString(field("wire api", kit.CardValue.Render(pr.WireAPI)+kit.StHint.Render("  (Codex)"), inner))
		}
		b.WriteString("\n")
	}
	indent := strings.Repeat(" ", 4)
	nameW := 0
	for _, st := range m.statuses {
		nameW = max(nameW, lipgloss.Width(st.AgentName))
	}
	for i, st := range m.statuses {
		mark, label := agentState(st, pr, ok)
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", nameW, st.AgentName))
		key := ""
		if ok {
			key = components.Keycap(fmt.Sprintf("%d", i+1)) + " "
		}
		b.WriteString(fmt.Sprintf("%s%s %s  %s\n", key, mark, name, label))
		// Com o perfil selecionado aplicado, a linha repetiria o card acima.
		if cur := currentLine(st); cur != "" && !(ok && st.Profile == pr.Name) {
			b.WriteString(kit.Wrap(cur, inner, indent) + "\n")
		}
		b.WriteString(kit.Wrap(kit.StHint.Render(core.Tilde(st.File, m.svc.home)), inner, indent) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// agentState é o marcador e o rótulo do agente em relação ao perfil
// selecionado (ou só o estado dele, sem perfil).
func agentState(st Status, pr agent.ProviderProfile, selected bool) (string, string) {
	switch {
	case st.Err != "":
		return kit.StErr.Render("!"), kit.StErr.Render(st.Err)
	case !st.Installed:
		return kit.StOff.Render("–"), kit.StOff.Render("CLI não instalado")
	case selected && st.Profile == pr.Name:
		return kit.StOn.Render("●"), kit.StOn.Render("aplicado")
	case st.Active && st.Profile != "":
		return kit.StShared.Render("◆"), kit.StShared.Render("usa o perfil " + st.Profile)
	case st.Active:
		return kit.StWarn.Render("◆"), kit.StWarn.Render("provedor configurado fora do lazyagents")
	}
	return kit.StOff.Render("○"), kit.StOff.Render("padrão do agente")
}

// currentLine descreve o provedor ativo no agente (endpoint, modelo, token),
// sempre a partir da leitura redigida da config viva.
func currentLine(st Status) string {
	if !st.Active {
		return ""
	}
	a := st.Applied
	var parts []string
	if a.BaseURL != "" {
		parts = append(parts, kit.CardValue.Render(a.BaseURL))
	}
	if a.Model != "" {
		parts = append(parts, kit.CardLabel.Render("modelo ")+kit.CardValue.Render(a.Model))
	}
	switch {
	case a.EnvKey != "":
		parts = append(parts, kit.CardLabel.Render("token em $"+a.EnvKey))
	case a.HasToken:
		parts = append(parts, kit.StOn.Render("token ✓"))
	}
	return strings.Join(parts, kit.StHint.Render(" · "))
}

// field é um rótulo com o valor alinhado; valor comprido (endpoint) quebra na
// coluna do valor, sem perder o fim.
func field(label, value string, inner int) string {
	pad := strings.Repeat(" ", 10)
	wrapped := kit.Wrap(value, inner, pad)
	return kit.CardLabel.Render(fmt.Sprintf("%-10s", label)) + strings.TrimPrefix(wrapped, pad) + "\n"
}

func orDash(s string) string { return orDefault(s, "—") }

func orDefault(s, def string) string {
	if s == "" {
		return kit.StHint.Render(def)
	}
	return kit.CardValue.Render(s)
}

// tokenLabel diz se há token e onde, sem nunca mostrar o valor.
func tokenLabel(p agent.ProviderProfile) string {
	switch {
	case p.EnvKey != "" && p.HasToken:
		return kit.StOn.Render("salvo") + kit.StHint.Render(" · variável $"+p.EnvKey)
	case p.EnvKey != "":
		return kit.CardValue.Render("$" + p.EnvKey)
	case p.HasToken:
		return kit.StOn.Render("salvo") + kit.StHint.Render(" · mascarado")
	}
	return kit.StHint.Render("sem token")
}
