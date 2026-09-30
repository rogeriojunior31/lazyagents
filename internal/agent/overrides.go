package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigOverride is an environment variable an agent CLI reads to move its
// files. The adapter honors the same one, so lazyagents looks where the CLI
// does; each was checked against the CLI itself, never assumed.
type ConfigOverride struct {
	Agent string // agent id
	Var   string
	Moves string // what the variable relocates, for the docs
}

// ConfigOverrides lists every variable the adapters read. Only absolute or
// ~-prefixed values count: a relative one is ignored (XDG calls it invalid,
// and where the CLI would resolve it is unknown).
var ConfigOverrides = []ConfigOverride{
	{"claude-code", "CLAUDE_CONFIG_DIR", "`~/.claude`: sessions, skills, settings, hooks, login"},
	{"codex", "CODEX_HOME", "`~/.codex`: sessions, skills, `config.toml`, hooks"},
	{"opencode", "XDG_CONFIG_HOME", "`~/.config`: `opencode/` config and skills"},
	{"opencode", "XDG_DATA_HOME", "`~/.local/share`: `opencode/opencode.db`"},
	{"opencode", "OPENCODE_DB", "the session database file itself"},
	{"hermes-agent", "HERMES_HOME", "`~/.hermes` (or the `hermes profile use` profile): skills and `config.yaml`"},
	{"pi", "PI_CODING_AGENT_DIR", "`~/.pi/agent`: everything"},
	{"pi", "PI_CODING_AGENT_SESSION_DIR", "`<agent dir>/sessions`"},
}

// envPath reads an override from the environment (see ConfigOverrides).
func envPath(home, name string) string { return expandPath(home, os.Getenv(name)) }

// expandPath resolves a leading ~ against home and cleans the path; a relative
// or empty one gives "". Clean matters: the index prunes deleted sessions by
// path prefix, which a trailing slash would break.
func expandPath(home, p string) string {
	switch {
	case p == "~" || strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[1:])
	case filepath.IsAbs(p):
		return filepath.Clean(p)
	}
	return ""
}
