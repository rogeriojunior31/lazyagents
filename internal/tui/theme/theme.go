// Package theme centraliza a paleta e os tokens de cor da TUI. É o ÚNICO lugar
// com literais de cor — todo o resto de internal/tui importa daqui, para que um
// refino de paleta seja uma edição só. Paleta base: Tokyo Night.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Tokens semânticos. Preferir estes aos hex — o significado sobrevive a troca
// de paleta (ex.: BorderFocus continua "borda do painel ativo" mesmo mudando a
// cor).
var (
	Primary     = lipgloss.Color("#7aa2f7") // azul — destaque, ativo, seleção
	Accent      = lipgloss.Color("#7dcfff") // ciano — realce secundário (títulos md)
	Subtle      = lipgloss.Color("#565f89") // cinza-azulado — hints, inativo
	Bg          = lipgloss.Color("#1a1b26") // fundo escuro — texto invertido
	Text        = lipgloss.Color("#c0caf5") // texto principal
	Sel         = lipgloss.Color("#292e42") // fundo da linha selecionada (bg_highlight)
	Border      = lipgloss.Color("#3b3d57") // bordas e separadores (painel inativo)
	BorderFocus = Primary                   // borda do painel focado
	OK          = lipgloss.Color("#9ece6a") // verde — ativo/sucesso
	Warn        = lipgloss.Color("#e0af68") // âmbar — local/atenção
	Err         = lipgloss.Color("#f7768e") // vermelho — erro
)

// agentColors fixa a cor de cada agente conhecido. Agente novo sem entrada
// recebe uma cor estável da paleta de fallback (hash do ID) — adicionar um
// adapter nunca exige editar a TUI.
var agentColors = map[string]color.Color{
	"claude-code": Warn,
	"codex":       Text,
	"gemini-cli":  Primary,
	"opencode":    OK,
}

var agentFallback = []color.Color{Accent, Primary, OK, Warn, Err}

// AgentColor devolve a cor de destaque do agente id.
func AgentColor(id string) color.Color {
	if c, ok := agentColors[id]; ok {
		return c
	}
	h := 0
	for _, r := range id {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return agentFallback[h%len(agentFallback)]
}
