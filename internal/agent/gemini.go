package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Gemini adapts Gemini CLI. Skills in ~/.gemini/skills (user scope) and
// ~/.agents/skills (shared); sessions in ~/.gemini/{history,tmp}/<proj>/chats.
type Gemini struct {
	Home string
	Look func(string) (string, error)
}

func NewGemini(home string) *Gemini { return &Gemini{Home: home, Look: exec.LookPath} }

func (g *Gemini) configDir() string { return filepath.Join(g.Home, ".gemini") }

func (g *Gemini) Detect() Agent {
	bin, _ := g.Look("gemini")
	a := Agent{
		ID:         "gemini-cli",
		Name:       "Gemini CLI",
		Short:      "G",
		Installed:  bin != "" || dirExists(g.configDir()),
		ManagedDir: filepath.Join(g.configDir(), "skills"),
	}
	a.ReadDirs = []string{a.ManagedDir, filepath.Join(g.Home, ".agents", "skills")}
	if bin != "" {
		a.Version = version(bin)
		a.Detail = bin
	} else if a.Installed {
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", g.configDir())
	} else {
		a.Detail = DetailNotInstalled
	}
	return a
}

// projectsMap inverts ~/.gemini/projects.json ({"projects":{path:name}}) into
// name→path, to resolve each session's cwd.
func (g *Gemini) projectsMap() map[string]string {
	data, err := os.ReadFile(filepath.Join(g.configDir(), "projects.json"))
	if err != nil {
		return nil
	}
	var pj struct {
		Projects map[string]string `json:"projects"`
	}
	if json.Unmarshal(data, &pj) != nil {
		return nil
	}
	byName := make(map[string]string, len(pj.Projects))
	for path, name := range pj.Projects {
		byName[name] = path
	}
	return byName
}

type geminiMeta struct {
	SessionID string `json:"sessionId"`
}

func (g *Gemini) ListSessions() ([]Session, error) {
	byName := g.projectsMap()
	seen := make(map[string]bool)
	var out []Session
	// history/ holds saved chats, tmp/ the current one; history first so the
	// dedupe drops tmp duplicates.
	for _, base := range []string{"history", "tmp"} {
		baseDir := filepath.Join(g.configDir(), base)
		projects, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, p := range projects {
			if !p.IsDir() {
				continue
			}
			chats := filepath.Join(baseDir, p.Name(), "chats")
			files, err := os.ReadDir(chats)
			if err != nil {
				continue
			}
			for _, f := range files {
				name := f.Name()
				if f.IsDir() || !strings.HasPrefix(name, "session-") {
					continue
				}
				info, err := f.Info()
				if err != nil {
					continue
				}
				path := filepath.Join(chats, name)
				s := Session{
					AgentID:   "gemini-cli",
					AgentName: "Gemini CLI",
					Path:      path,
					CWD:       byName[p.Name()],
					MTime:     info.ModTime(),
				}
				lines := firstLines(path, 40)
				if len(lines) > 0 {
					var m geminiMeta
					if json.Unmarshal(lines[0], &m) == nil {
						s.ID = m.SessionID
					}
				}
				for _, line := range lines {
					if s.Title = cleanTitle(looseUserText(line), 80); s.Title != "" {
						break
					}
				}
				if s.ID == "" {
					s.ID = strings.TrimSuffix(strings.TrimPrefix(name, "session-"), filepath.Ext(name))
				}
				if s.Title == "" {
					s.Title = "(no prompt)"
				}
				if seen[s.ID] {
					continue
				}
				seen[s.ID] = true
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	return out, nil
}

func (g *Gemini) ResumeCmd(s Session) ([]string, string, bool) {
	dir := s.CWD
	if !dirExists(dir) {
		dir = g.Home
	}
	return []string{"gemini", "--resume", s.ID}, dir, true
}

func (g *Gemini) ID() string { return "gemini-cli" }

func (g *Gemini) Transcript(s Session) ([]Entry, error) {
	return jsonlTranscript(s.Path)
}

func (g *Gemini) DeleteSession(s Session, backupsDir string) error {
	return deleteSessionFile(s.Path, backupsDir)
}
