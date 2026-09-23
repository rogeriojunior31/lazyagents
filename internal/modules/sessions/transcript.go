package sessions

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// maxChatWidth é a largura de leitura do transcript — mesmo com o terminal
// largo, a conversa não estica além disso.
const maxChatWidth = 100

// turn é uma vez de falar: a mensagem do usuário, ou tudo o que o agente
// disse e chamou até o próximo prompt (texto e ferramentas, em ordem).
type turn struct {
	user    bool
	entries []agent.Entry
}

func turns(entries []agent.Entry) []turn {
	var out []turn
	for _, e := range entries {
		user := e.Role == agent.RoleUser
		if len(out) > 0 && out[len(out)-1].user == user {
			out[len(out)-1].entries = append(out[len(out)-1].entries, e)
			continue
		}
		out = append(out, turn{user: user, entries: []agent.Entry{e}})
	}
	return out
}

// transcriptView é o transcript renderizado e onde começa cada prompt do
// usuário (linhas), para n/N pularem entre eles.
type transcriptView struct {
	content string
	prompts []int
	stats   transcriptStats
}

// transcriptStats conta prompts, turnos do agente e chamadas de ferramenta.
type transcriptStats struct{ prompts, replies, tools int }

// chatColumn é a coluna da conversa dentro de width: até maxChatWidth,
// centralizada. Devolve a largura e o recuo à esquerda.
func chatColumn(width int) (w, pad int) {
	w = max(20, min(width, maxChatWidth))
	return w, max(0, (width-w)/2)
}

// renderTranscript formata a conversa como chat, numa coluna centralizada:
// prompts do usuário em balões à direita, turnos do agente em balões à
// esquerda com a borda na cor dele, texto em Markdown. Chamadas de
// ferramenta aparecem dentro do balão, uma por linha (showTools) ou
// resumidas por trecho ("⚙ 4 chamadas · Bash ×3, Read").
func renderTranscript(entries []agent.Entry, width int, s agent.Session, showTools bool) transcriptView {
	var v transcriptView
	if len(entries) == 0 {
		v.content = kit.StHint.Render("(transcript vazio ou em formato desconhecido)")
		return v
	}
	w, pad := chatColumn(width)
	agentName := s.AgentName
	if agentName == "" {
		agentName = "agente"
	}
	userColor, agentColor := theme.Primary, theme.AgentColor(s.AgentID)
	bubble := func(c color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).Padding(0, 1)
	}

	var lines []string
	add := func(block string) {
		for _, ln := range strings.Split(block, "\n") {
			lines = append(lines, strings.Repeat(" ", pad)+ln)
		}
	}
	for i, t := range turns(entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		if t.user {
			v.stats.prompts++
			v.prompts = append(v.prompts, len(lines))
			// Balão do tamanho do texto, até 3/4 da coluna, encostado à direita.
			inner := max(10, w*3/4-4)
			body := strings.Join(turnBlocks(t, inner, showTools, &v.stats), "\n")
			body = lipgloss.NewStyle().MaxWidth(inner).Render(body)
			label := kit.StHint.Render(fmt.Sprintf("#%d  ", v.stats.prompts)) +
				lipgloss.NewStyle().Foreground(userColor).Bold(true).Render("Você")
			add(lipgloss.PlaceHorizontal(w, lipgloss.Right, label))
			add(lipgloss.PlaceHorizontal(w, lipgloss.Right, bubble(userColor).Render(body)))
			continue
		}
		v.stats.replies++
		body := strings.Join(turnBlocks(t, w-4, showTools, &v.stats), "\n")
		add(lipgloss.NewStyle().Foreground(agentColor).Bold(true).Render(agentName))
		add(bubble(agentColor).Render(body)) // do tamanho do texto, até a coluna
	}
	v.content = strings.Join(lines, "\n")
	return v
}

// turnBlocks devolve os blocos de um turno já quebrados em width colunas:
// texto em Markdown e as ferramentas, cada trecho contíguo junto.
func turnBlocks(t turn, width int, showTools bool, st *transcriptStats) []string {
	var out []string
	var run []string // ferramentas seguidas, ainda não emitidas
	flush := func() {
		if len(run) == 0 {
			return
		}
		st.tools += len(run)
		if showTools {
			for _, call := range run {
				out = append(out, toolLine(call, width))
			}
		} else {
			out = append(out, kit.StHint.Render(ansi.Truncate(toolSummary(run), width, "…")))
		}
		run = nil
	}
	for _, e := range t.entries {
		if e.Role == agent.RoleTool {
			run = append(run, e.Text)
			continue
		}
		flush()
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, kit.RenderChat(e.Text, width))
	}
	flush()
	return out
}

// toolLine é uma chamada: nome em destaque e o argumento esmaecido.
func toolLine(call string, width int) string {
	name, arg, _ := strings.Cut(call, " · ")
	line := kit.StShared.Render("⚙ "+name) + kit.StHint.Render("  "+arg)
	return ansi.Truncate(line, width, "…")
}

// toolSummary resume um trecho de chamadas: "⚙ 4 chamadas · Bash ×3, Read".
func toolSummary(run []string) string {
	if len(run) == 1 {
		return "⚙ " + run[0]
	}
	var order []string
	count := map[string]int{}
	for _, call := range run {
		name, _, _ := strings.Cut(call, " · ")
		if count[name] == 0 {
			order = append(order, name)
		}
		count[name]++
	}
	parts := make([]string, len(order))
	for i, n := range order {
		parts[i] = n
		if count[n] > 1 {
			parts[i] += fmt.Sprintf(" ×%d", count[n])
		}
	}
	return fmt.Sprintf("⚙ %d chamadas · %s", len(run), strings.Join(parts, ", "))
}
