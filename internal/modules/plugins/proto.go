// Package plugin executa plugins externos: binários em <ConfigDir>/plugins que
// falam JSON Lines por stdin/stdout e podem virar abas da TUI, subcomandos da
// CLI e seções do doctor. Este arquivo é o contrato do fio; docs/plugins.md
// é a documentação. Mudança incompatível = bump de Protocol.
package plugins

import (
	"encoding/json"
	"strings"
)

// Protocol é a versão do protocolo enviada no init.
const Protocol = 1

// MaxLine limita cada linha JSON (nas duas direções) e a saída capturada de exec.
const MaxLine = 1 << 20

// Msg é a união de todas as mensagens do protocolo; Type discrimina e os
// demais campos são omitidos quando vazios.
type Msg struct {
	Type string `json:"type"`

	// init (host → plugin)
	Protocol   int             `json:"protocol,omitempty"`
	ID         string          `json:"id,omitempty"`
	Home       string          `json:"home,omitempty"`
	ConfigDir  string          `json:"configDir,omitempty"`
	DataDir    string          `json:"dataDir,omitempty"`
	LibraryDir string          `json:"libraryDir,omitempty"`
	Theme      *Theme          `json:"theme,omitempty"`
	Config     json.RawMessage `json:"config,omitempty"` // seção <id>: do config.yaml

	// init / resize
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	// key / paste / mouse
	Key   string `json:"key,omitempty"` // tea.KeyPressMsg.String(): "a", "enter", "space", "ctrl+x"
	Text  string `json:"text,omitempty"`
	Mouse *Mouse `json:"mouse,omitempty"`

	// agents
	Agents []Agent `json:"agents,omitempty"`

	// manifest (plugin → host)
	Title    string      `json:"title,omitempty"`
	Help     []HelpGroup `json:"help,omitempty"`
	Commands []Command   `json:"commands,omitempty"`
	Doctor   bool        `json:"doctor,omitempty"` // suporta `<bin> doctor`

	// frame (plugin → host)
	View      string `json:"view,omitempty"`
	Count     *int   `json:"count,omitempty"` // ausente = sem contador na aba
	Capturing bool   `json:"capturing,omitempty"`

	// command (host → plugin): entrada da paleta escolhida
	Name string `json:"name,omitempty"`

	// exec (plugin → host) / exec_result (host → plugin)
	ExecID      int      `json:"execId,omitempty"`
	Argv        []string `json:"argv,omitempty"`
	Dir         string   `json:"dir,omitempty"`
	Interactive bool     `json:"interactive,omitempty"`
	Code        int      `json:"code,omitempty"`
	Stdout      string   `json:"stdout,omitempty"`
	Stderr      string   `json:"stderr,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// Theme é o tema ativo: id e cores em hex, por token (Primary, Bg, Info…) e
// por papel do SP Night (ui.accent, syntax.string, ansi.red…).
type Theme struct {
	ID     string            `json:"id"`
	Colors map[string]string `json:"colors"`
}

// Agent é o DTO de agent.Agent no fio (nomes estáveis, minúsculos).
type Agent struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Installed  bool     `json:"installed"`
	Version    string   `json:"version,omitempty"`
	ManagedDir string   `json:"managedDir,omitempty"`
	ReadDirs   []string `json:"readDirs,omitempty"`
}

// HelpGroup é um bloco do modal de ajuda (?): título + pares [tecla, descrição].
type HelpGroup struct {
	Title string      `json:"title"`
	Keys  [][2]string `json:"keys"`
}

// Command é uma entrada da paleta (:) contribuída pelo plugin, prefixada pelo id.
type Command struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// Mouse é um evento de mouse já em coordenadas do corpo da aba.
type Mouse struct {
	Kind   string `json:"kind"` // "wheel" | "click"
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button string `json:"button,omitempty"`
}

// CleanView sanitiza o view de um plugin: mantém texto, quebras de linha e
// SGR (ESC [ … m, as cores); remove qualquer outra sequência de escape
// (cursor, limpar tela, OSC) e controles C0, que quebrariam o layout do root.
// Tab vira 4 espaços.
func CleanView(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0x1b:
			i += skipEscape(s[i:], &b) - 1
		case c == '\n':
			b.WriteByte(c)
		case c == '\t':
			b.WriteString("    ")
		case c < 0x20 || c == 0x7f:
			// controle C0 / DEL: descarta
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// skipEscape consome a sequência de escape no início de s (s[0] == ESC),
// escrevendo-a em b só quando é SGR. Devolve quantos bytes consumiu (≥ 1).
func skipEscape(s string, b *strings.Builder) int {
	if len(s) < 2 {
		return 1
	}
	switch s[1] {
	case '[': // CSI: parâmetros 0x30–0x3F, intermediários 0x20–0x2F, final 0x40–0x7E
		i := 2
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
			i++
		}
		if i < len(s) && s[i] >= 0x40 && s[i] <= 0x7e {
			if s[i] == 'm' {
				b.WriteString(s[:i+1])
			}
			return i + 1
		}
		return i
	case ']': // OSC: até BEL ou ST (ESC \)
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	default: // ESC + um byte (ex.: ESC 7)
		return 2
	}
}
