// Package plugins runs external plugins: binaries in <ConfigDir>/plugins that speak
// JSON Lines over stdin/stdout and can become TUI tabs, CLI subcommands and doctor
// sections. This file is the wire contract (documented in docs/plugins.md); an
// incompatible change bumps Protocol.
package plugins

import (
	"encoding/json"
	"strings"
)

// Protocol is the protocol version sent in init.
const Protocol = 1

// MaxLine caps each JSON line (both directions) and captured exec output.
const MaxLine = 1 << 20

// Msg is the union of every protocol message; Type discriminates, and empty
// fields are omitted.
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
	Config     json.RawMessage `json:"config,omitempty"` // the <id>: section of config.yaml

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
	Doctor   bool        `json:"doctor,omitempty"` // supports `<bin> doctor`

	// frame (plugin → host)
	View      string `json:"view,omitempty"`
	Count     *int   `json:"count,omitempty"` // absent = no counter on the tab
	Capturing bool   `json:"capturing,omitempty"`

	// command (host → plugin): the palette entry picked
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

// Theme is the active theme: id and hex colors by token (Primary, Bg, Info…) and
// by SP Night role (ui.accent, syntax.string, ansi.red…).
type Theme struct {
	ID     string            `json:"id"`
	Colors map[string]string `json:"colors"`
}

// Agent is the wire DTO of agent.Agent (stable lowercase names).
type Agent struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Installed  bool     `json:"installed"`
	Version    string   `json:"version,omitempty"`
	ManagedDir string   `json:"managedDir,omitempty"`
	ReadDirs   []string `json:"readDirs,omitempty"`
}

// HelpGroup is a help modal (?) block: title + [key, description] pairs.
type HelpGroup struct {
	Title string      `json:"title"`
	Keys  [][2]string `json:"keys"`
}

// Command is a palette (:) entry contributed by the plugin, prefixed by its id.
type Command struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// Mouse is a mouse event in tab-body coordinates.
type Mouse struct {
	Kind   string `json:"kind"` // "wheel" | "click"
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button string `json:"button,omitempty"`
}

// CleanView sanitizes a plugin view: keeps text, newlines and SGR colors; drops
// every other escape sequence (cursor, clear screen, OSC) and C0 controls, which
// would break the root layout. Tab becomes 4 spaces.
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
			// C0 control / DEL: drop
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// skipEscape consumes the escape sequence at the start of s (s[0] == ESC),
// writing it to b only if it is SGR. Returns the bytes consumed (≥ 1).
func skipEscape(s string, b *strings.Builder) int {
	if len(s) < 2 {
		return 1
	}
	switch s[1] {
	case '[': // CSI: params 0x30–0x3F, intermediates 0x20–0x2F, final 0x40–0x7E
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
	case ']': // OSC: up to BEL or ST (ESC \)
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	default: // ESC + one byte (e.g. ESC 7)
		return 2
	}
}
