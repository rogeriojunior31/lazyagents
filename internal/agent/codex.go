package agent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Codex adapts OpenAI Codex CLI. Skills in the cross-agent ~/.agents/skills
// (also reads ~/.codex/skills); sessions in
// ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
type Codex struct {
	Home string
	Look func(string) (string, error)
	// Index remembers what was read of each rollout (nil = memory only).
	Index     *Index
	indexOnce sync.Once
}

func (c *Codex) index() *Index {
	c.indexOnce.Do(func() {
		if c.Index == nil {
			c.Index = NewIndex("")
		}
	})
	return c.Index
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
		SharedNote: "~/.agents/skills is also read by Gemini and OpenCode",
	}
	if bin != "" {
		a.Version = version(bin)
		a.Detail = bin
	} else if a.Installed {
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", c.configDir())
	} else {
		a.Detail = DetailNotInstalled
	}
	return a
}

// codexLine parses a rollout line: the first is a session_meta with id/cwd in
// the payload; messages are response_item with input_text blocks.
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
			return nil //nolint:nilerr // best-effort: unreadable entries are skipped
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
			// fallback: rollout-2026-07-06T12-00-00-<uuid>.jsonl ends in the UUID
			base := strings.TrimSuffix(d.Name(), ".jsonl")
			if len(base) >= 36 {
				s.ID = base[len(base)-36:]
			} else {
				s.ID = base
			}
		}
		if s.Title == "" {
			s.Title = "(no prompt)"
		}
		out = append(out, s)
		return nil
	})
	if err != nil {
		return out, err
	}
	// usage and limits come from the whole rollout: the index reads each file
	// once, then only what was appended
	idx := c.index()
	paths := make([]string, len(out))
	keep := make(map[string]bool, len(out))
	for i, s := range out {
		paths[i], keep[s.Path] = s.Path, true
	}
	idx.refreshAll(paths, codexIndexLine)
	idx.retain(root+string(filepath.Separator), keep)
	idx.save()
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

func (c *Codex) ID() string { return "codex" }

func (c *Codex) Transcript(s Session) ([]Entry, error) {
	return jsonlTranscript(s.Path)
}

func (c *Codex) DeleteSession(s Session, backupsDir string) error {
	return deleteSessionFile(s.Path, backupsDir)
}
