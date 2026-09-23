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

// transcriptStats conta prompts, turnos do agente, chamadas de ferramenta e
// blocos de raciocínio.
type transcriptStats struct{ prompts, replies, tools, thoughts int }

// chatColumn é a coluna da conversa dentro de width: até maxChatWidth,
// centralizada. Devolve a largura e o recuo à esquerda.
func chatColumn(width int) (w, pad int) {
	w = max(20, min(width, maxChatWidth))
	return w, max(0, (width-w)/2)
}

// renderTranscript formata a conversa como chat, numa coluna centralizada:
// prompts do usuário em balões à direita, turnos do agente em balões à
// esquerda com a borda na cor dele. Dentro do balão, raciocínio (💭),
// fala e comandos (❯) têm estilos próprios — ver turnBlocks.
func renderTranscript(entries []agent.Entry, width int, s agent.Session, o transcriptOpts) transcriptView {
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
			body := strings.Join(turnBlocks(t, inner, o, &v.stats), "\n")
			body = lipgloss.NewStyle().MaxWidth(inner).Render(body)
			label := kit.StHint.Render(fmt.Sprintf("#%d  ", v.stats.prompts)) +
				lipgloss.NewStyle().Foreground(userColor).Bold(true).Render("Você")
			add(lipgloss.PlaceHorizontal(w, lipgloss.Right, label))
			add(lipgloss.PlaceHorizontal(w, lipgloss.Right, bubble(userColor).Render(body)))
			continue
		}
		v.stats.replies++
		body := strings.Join(turnBlocks(t, w-4, o, &v.stats), "\n")
		add(lipgloss.NewStyle().Foreground(agentColor).Bold(true).Render(agentName))
		add(bubble(agentColor).Render(body)) // do tamanho do texto, até a coluna
	}
	v.content = strings.Join(lines, "\n")
	return v
}

// transcriptOpts diz o que fica expandido no leitor.
type transcriptOpts struct {
	tools    bool // t: um comando por linha (senão, resumo por trecho)
	thinking bool // r: raciocínio inteiro (senão, só a primeira linha)
	home     string // encurta caminhos dos comandos para ~
}

var (
	thinkStyle = lipgloss.NewStyle().Foreground(theme.Subtle).Italic(true)
	cmdMark    = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
)

// turnBlocks devolve os blocos de um turno já quebrados em width colunas, na
// ordem em que o agente os produziu, com uma linha em branco sempre que o
// tipo muda: raciocínio, fala e comandos nunca se confundem.
func turnBlocks(t turn, width int, o transcriptOpts, st *transcriptStats) []string {
	var out []string
	var run []string // comandos seguidos, ainda não emitidos
	last := ""       // papel do último bloco emitido
	emit := func(role, block string) {
		if last != "" && (role != last || role == agent.RoleAssistant) {
			out = append(out, "")
		}
		out, last = append(out, block), role
	}
	flush := func() {
		if len(run) == 0 {
			return
		}
		st.tools += len(run)
		if o.tools {
			lines := make([]string, len(run))
			for i, call := range run {
				lines[i] = toolLine(call, width)
			}
			emit(agent.RoleTool, strings.Join(lines, "\n"))
		} else {
			emit(agent.RoleTool, toolSummary(run, width))
		}
		run = nil
	}
	for _, e := range t.entries {
		switch e.Role {
		case agent.RoleTool:
			call := e.Text
			if o.home != "" {
				call = strings.ReplaceAll(call, o.home+"/", "~/")
			}
			run = append(run, call)
			continue
		case agent.RoleThinking:
			flush()
			st.thoughts++
			emit(agent.RoleThinking, thinkingBlock(e.Text, width, o.thinking))
			continue
		}
		flush()
		emit(agent.RoleAssistant, kit.RenderChat(e.Text, width))
	}
	flush()
	return out
}

// thinkingBlock é o raciocínio esmaecido: a primeira linha com "…" quando
// recolhido, o texto inteiro atrás de uma régua pontilhada quando aberto.
func thinkingBlock(text string, width int, full bool) string {
	if !full {
		first, _, more := strings.Cut(strings.TrimSpace(text), "\n")
		line := "💭 " + first
		if more || lipgloss.Width(line) > width {
			line = ansi.Truncate(line, width-1, "") + "…"
		}
		return thinkStyle.Render(line)
	}
	body := lipgloss.NewStyle().Width(width - 2).Render(text)
	lines := []string{thinkStyle.Render("💭 raciocínio")}
	for _, ln := range strings.Split(body, "\n") {
		lines = append(lines, thinkStyle.Render("┆ "+strings.TrimRight(ln, " ")))
	}
	return strings.Join(lines, "\n")
}

// toolLine é um comando: marcador, nome da ferramenta e o argumento na cor
// de código.
func toolLine(call string, width int) string {
	name, arg, _ := strings.Cut(call, " · ")
	line := cmdMark.Render("❯ ") + kit.StShared.Bold(true).Render(name) + "  " + kit.MdCode.Render(arg)
	return ansi.Truncate(line, width, "…")
}

// toolSummary resume um trecho de comandos: "❯ 4 comandos · Bash ×3, Read";
// um só aparece inteiro.
func toolSummary(run []string, width int) string {
	if len(run) == 1 {
		return toolLine(run[0], width)
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
	line := cmdMark.Render("❯ ") + kit.StShared.Render(fmt.Sprintf("%d comandos", len(run))) +
		kit.StHint.Render(" · "+strings.Join(parts, ", "))
	return ansi.Truncate(line, width, "…")
}
