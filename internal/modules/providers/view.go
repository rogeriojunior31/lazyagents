package providers

import (
	"fmt"
	"net/url"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// narrowWidth é a largura abaixo da qual lista e detalhe empilham.
const narrowWidth = 76

func (m Tab) listWidth() int {
	if m.width < narrowWidth {
		return m.width
	}
	return max(30, m.width*2/5)
}

func (m Tab) detailWidth() int {
	if m.width < narrowWidth {
		return m.width
	}
	return max(24, m.width-m.listWidth()-2)
}

// bodyHeight é a altura do corpo, descontados hints e toast.
func (m Tab) bodyHeight() int { return max(6, m.height-2) }

// View limita tudo à largura da aba: rede de segurança para terminal estreito.
func (m Tab) View() string {
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).Render(m.view())
}

func (m Tab) view() string {
	if m.confirm != nil {
		return m.confirm.View()
	}
	if m.loading && len(m.statuses) == 0 {
		return kit.StHint.Render("  lendo perfis e configs…")
	}
	if len(m.statuses) == 0 {
		return kit.StHint.Render("  nenhum agente instalado suporta troca de provedor.")
	}

	bodyH := m.bodyHeight()
	var body string
	if m.width < narrowWidth {
		listH := min(bodyH/2, 3*max(1, len(m.profiles))+3)
		body = lipgloss.JoinVertical(lipgloss.Left,
			m.listPanel(m.width, max(4, listH)),
			m.detailPanel(m.width, max(4, bodyH-listH)))
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.listPanel(m.listWidth(), bodyH), "  ",
			m.detailPanel(m.detailWidth(), bodyH))
	}

	hints := kit.Hints(m.width, [2]string{"x", "volta ao padrão"}, [2]string{"r", "recarrega"}, [2]string{"?", "atalhos"})
	if len(m.profiles) > 0 {
		hints = kit.Hints(m.width,
			[2]string{"1-9", "aplica/remove no agente"},
			[2]string{"space", "em todos"},
			[2]string{"x", "volta ao padrão"},
			[2]string{"d", "apaga perfil"},
			[2]string{"?", "atalhos"})
	}
	out := []string{body, hints}
	if m.toast != "" {
		out = append(out, components.Toast(m.toast, m.toastErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// listPanel é a biblioteca de perfis: nome, onde está aplicado e o endpoint.
func (m Tab) listPanel(w, h int) string {
	p := components.Panel{Title: fmt.Sprintf("PERFIS   %d", len(m.profiles)), Focused: true, Width: w, Height: h}
	inner := p.ContentWidth()
	if len(m.profiles) == 0 {
		return p.Render("\n" + kit.StHint.Render("Nenhum perfil ainda."))
	}
	per := max(1, (p.ContentHeight()-1)/3) // 3 linhas por perfil
	start, end := kit.Window(m.cursor, len(m.profiles), per)
	lines := []string{""}
	for i := start; i < end; i++ {
		pr := m.profiles[i]
		lines = append(lines, strings.Split(kit.ListRow(inner, i == m.cursor, pr.Name, m.agentMarks(pr), profileSummary(pr)), "\n")...)
		lines = append(lines, "")
	}
	if end < len(m.profiles) {
		lines[len(lines)-1] = kit.StHint.Render(fmt.Sprintf("  ↓ mais %d", len(m.profiles)-end))
	}
	return p.Render(strings.Join(lines, "\n"))
}

// agentMarks marca, por agente, se o perfil é o aplicado ali.
func (m Tab) agentMarks(p agent.ProviderProfile) string {
	var out []string
	for _, st := range m.statuses {
		if st.Profile == p.Name {
			out = append(out, kit.StOn.Render("●"))
		} else {
			out = append(out, kit.StOff.Render("○"))
		}
	}
	return strings.Join(out, " ")
}

// profileSummary é a segunda linha da lista: host do endpoint e modelo.
func profileSummary(p agent.ProviderProfile) string {
	var parts []string
	if p.BaseURL != "" {
		parts = append(parts, endpointHost(p.BaseURL))
	}
	if p.Model != "" {
		parts = append(parts, p.Model)
	}
	if len(parts) == 0 {
		return "sem endpoint nem modelo"
	}
	return strings.Join(parts, " · ")
}

// endpointHost encurta a URL para o host, que é o que distingue um provedor
// de outro numa linha estreita.
func endpointHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func (m Tab) detailPanel(w, h int) string {
	p := components.Panel{Title: "SOBRE O PERFIL", Width: w, Height: h}
	if _, ok := m.current(); !ok {
		p.Title = "AGENTES"
	}
	lines := strings.Split(m.detailContent(p.ContentWidth()), "\n")
	if visible := p.ContentHeight(); len(lines) > visible && visible > 1 {
		lines = append(lines[:visible-1], kit.StHint.Render("…"))
	}
	return p.Render(strings.Join(lines, "\n"))
}

func (m Tab) detailContent(inner int) string {
	var b strings.Builder
	pr, ok := m.current()
	if ok {
		b.WriteString(kit.StTitle.Render(pr.Name) + "\n\n")
		b.WriteString(field("endpoint", orDash(pr.BaseURL)))
		b.WriteString(field("modelo", orDefault(pr.Model, "padrão do agente")))
		b.WriteString(field("token", tokenLabel(pr)))
		if pr.WireAPI != "" {
			b.WriteString(field("wire api", kit.CardValue.Render(pr.WireAPI)+kit.StHint.Render("  (Codex)")))
		}
		b.WriteString("\n" + kit.StHint.Render("NOS AGENTES") + "\n")
	} else {
		b.WriteString(kit.StText.Render("Nenhum perfil na biblioteca. Crie um pela CLI:") + "\n")
		b.WriteString(kit.CardValue.Render("  lazyagents provider add trabalho --base-url https://… --token -") + "\n")
		b.WriteString(kit.StHint.Render("  --token - lê o token da entrada padrão, sem passar pelo histórico do shell.") + "\n\n")
		b.WriteString(kit.StHint.Render("APLICADO AGORA") + "\n")
	}

	nameW := 0
	for _, st := range m.statuses {
		nameW = max(nameW, lipgloss.Width(st.AgentName))
	}
	indent := strings.Repeat(" ", 6)
	for i, st := range m.statuses {
		mark, label := agentState(st, pr, ok)
		name := lipgloss.NewStyle().Foreground(theme.AgentColor(st.AgentID)).Render(fmt.Sprintf("%-*s", nameW, st.AgentName))
		key := "  "
		if ok {
			key = components.Keycap(fmt.Sprintf("%d", i+1))
		}
		b.WriteString(fmt.Sprintf("%s %s %s  %s\n", key, mark, name, label))
		// Com o perfil selecionado aplicado, a linha repetiria o card acima.
		if cur := currentLine(st); cur != "" && !(ok && st.Profile == pr.Name) {
			b.WriteString(indent + ansi.Truncate(cur, max(10, inner-6), "…") + "\n")
		}
		b.WriteString(indent + kit.StHint.Render(ansi.Truncate(core.Tilde(st.File, m.svc.home), max(10, inner-6), "…")) + "\n")
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

func field(label, value string) string {
	return kit.CardLabel.Render(fmt.Sprintf("%-10s", label)) + value + "\n"
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
