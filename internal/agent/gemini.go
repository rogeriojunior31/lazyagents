package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Gemini adapts Gemini CLI. Skills in ~/.gemini/skills (user scope) and
// ~/.agents/skills (shared); sessions in ~/.gemini/{history,tmp}/<proj>/chats.
type Gemini struct {
	Home          string
	Look          func(string) (string, error)
	ProviderState string
	providerMem   *geminiProviderState
	Index         *Index
	indexOnce     sync.Once
}

func NewGemini(home string) *Gemini { return &Gemini{Home: home, Look: exec.LookPath} }

func (g *Gemini) nativeHome() string {
	if home := envPath(g.Home, "GEMINI_CLI_HOME"); home != "" {
		return home
	}
	return g.Home
}
func (g *Gemini) configDir() string { return filepath.Join(g.nativeHome(), ".gemini") }

func (g *Gemini) Detect() Agent {
	bin, _ := g.Look("gemini")
	a := Agent{
		ID:         "gemini-cli",
		Name:       "Gemini CLI",
		Short:      "G",
		Installed:  bin != "" || dirExists(g.configDir()),
		ManagedDir: filepath.Join(g.configDir(), "skills"),
	}
	a.SharedDir = filepath.Join(g.nativeHome(), ".agents", "skills")
	a.ReadDirs = []string{a.ManagedDir, a.SharedDir}
	if bin != "" {
		a.detectVersion(bin)
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

func (g *Gemini) ListSessions() ([]Session, error) {
	byName := g.projectsMap()
	seen := make(map[string]int)
	var out []Session
	// Read both locations before choosing the newest native snapshot of an ID.
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
				s.ID = strings.TrimSuffix(strings.TrimPrefix(name, "session-"), filepath.Ext(name))
				out = append(out, s)
			}
		}
	}
	paths := make([]string, 0, len(out))
	keep := map[string]bool{}
	for _, s := range out {
		paths = append(paths, s.Path)
		keep[s.Path] = true
	}
	idx := g.index()
	idx.retain(g.configDir()+string(filepath.Separator), keep)
	idx.refreshAll(paths, geminiIndexLine)
	idx.save()
	var sessions []Session
	for _, s := range out {
		if e, ok := idx.get(s.Path); ok {
			if e.GeminiSessionID != "" {
				s.ID = e.GeminiSessionID
			}
			s.Title = e.FirstPrompt
			if !e.GeminiUpdated.IsZero() {
				s.MTime = e.GeminiUpdated
			} else if !e.GeminiStarted.IsZero() {
				s.MTime = e.GeminiStarted
			}
		}
		if s.Title == "" {
			s.Title = "(no prompt)"
		}
		if i, ok := seen[s.ID]; ok {
			if s.MTime.After(sessions[i].MTime) {
				sessions[i] = s
			}
			continue
		}
		seen[s.ID] = len(sessions)
		sessions = append(sessions, s)
	}
	out = sessions
	sort.SliceStable(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
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
	return geminiTranscript(s.Path)
}

func (g *Gemini) DeleteSession(s Session, backupsDir string) error {
	return deleteSessionFile(s.Path, backupsDir)
}
