package agent

import (
	"encoding/json"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Codex adapta o OpenAI Codex CLI. Skills no dir padrão cross-agente
// ~/.agents/skills (com fallback de leitura em ~/.codex/skills); sessões em
// ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
type Codex struct {
	Home string
	Look func(string) (string, error)
}

func NewCodex(home string) *Codex { return &Codex{Home: home, Look: exec.LookPath} }

func (c *Codex) configDir() string   { return filepath.Join(c.Home, ".codex") }
func (c *Codex) sessionsDir() string { return filepath.Join(c.configDir(), "sessions") }

func (c *Codex) Detect() Agent {
	bin, _ := c.Look("codex")
	agentsDir := filepath.Join(c.Home, ".agents", "skills")
	a := Agent{
		ID:         "codex",
		Name:       "Codex",
		Short:      "X",
		Installed:  bin != "" || dirExists(c.configDir()),
		ManagedDir: agentsDir,
		ReadDirs:   []string{agentsDir, filepath.Join(c.configDir(), "skills")},
		SharedNote: "~/.agents/skills é lido também por Gemini e OpenCode",
	}
	if bin != "" {
		a.Version = version(bin)
		a.Detail = bin
	} else if a.Installed {
		a.Detail = "config em " + c.configDir() + " (binário fora do PATH)"
	} else {
		a.Detail = "não instalado"
	}
	return a
}

// codexLine cobre o rollout JSONL: a primeira linha é um session_meta com
// id/cwd no payload; mensagens vêm como response_item com blocos input_text.
type codexLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexMeta struct {
	ID  string `json:"id"`
	CWD string `json:"cwd"`
}

func (c *Codex) ListSessions() ([]Session, error) {
	var out []Session
	root := c.sessionsDir()
	if !dirExists(root) {
		return nil, nil
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil //nolint:nilerr // best-effort: entrada ilegível é pulada
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		s := Session{
			AgentID:   "codex",
			AgentName: "Codex",
			Path:      path,
			MTime:     info.ModTime(),
		}
		for _, line := range firstLines(path, 40) {
			var e codexLine
			if json.Unmarshal(line, &e) != nil {
				continue
			}
			if e.Type == "session_meta" && s.ID == "" {
				var m codexMeta
				if json.Unmarshal(e.Payload, &m) == nil {
					s.ID, s.CWD = m.ID, m.CWD
				}
			}
			if s.Title == "" {
				s.Title = cleanTitle(looseUserText(line), 80)
			}
			if s.ID != "" && s.Title != "" {
				break
			}
		}
		if s.ID == "" {
			// fallback: rollout-2026-07-06T12-00-00-<uuid>.jsonl → últimos 36
			// chars são o UUID
			base := strings.TrimSuffix(d.Name(), ".jsonl")
			if len(base) >= 36 {
				s.ID = base[len(base)-36:]
			} else {
				s.ID = base
			}
		}
		if s.Title == "" {
			s.Title = "(sem prompt)"
		}
		out = append(out, s)
		return nil
	})
	if err != nil {
		return out, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	return out, nil
}

func (c *Codex) ResumeCmd(s Session) ([]string, string, bool) {
	dir := s.CWD
	if !dirExists(dir) {
		dir = c.Home
	}
	return []string{"codex", "resume", s.ID}, dir, true
}

// ID implementa Adapter sem I/O.
func (c *Codex) ID() string { return "codex" }
