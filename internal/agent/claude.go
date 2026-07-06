package agent

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Claude adapta o Claude Code (CLI). Skills em ~/.claude/skills; sessões em
// ~/.claude/projects/<slug>/<sessionId>.jsonl.
type Claude struct {
	Home string
	Look func(string) (string, error) // injetável em teste
}

func NewClaude(home string) *Claude { return &Claude{Home: home, Look: exec.LookPath} }

func (c *Claude) configDir() string   { return filepath.Join(c.Home, ".claude") }
func (c *Claude) projectsDir() string { return filepath.Join(c.configDir(), "projects") }

func (c *Claude) Detect() Agent {
	bin, _ := c.Look("claude")
	a := Agent{
		ID:         "claude-code",
		Name:       "Claude Code",
		Short:      "C",
		Installed:  bin != "" || dirExists(c.configDir()),
		ManagedDir: filepath.Join(c.configDir(), "skills"),
	}
	a.ReadDirs = []string{a.ManagedDir}
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

// claudeLine cobre os campos usados das entradas do JSONL do Claude Code.
type claudeLine struct {
	Type    string `json:"type"`
	IsMeta  bool   `json:"isMeta"`
	CWD     string `json:"cwd"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func (c *Claude) ListSessions() ([]Session, error) {
	projects, err := os.ReadDir(c.projectsDir())
	if err != nil {
		return nil, nil // sem projetos = sem sessões, não é erro
	}
	var out []Session
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		projDir := filepath.Join(c.projectsDir(), p.Name())
		files, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(projDir, f.Name())
			info, err := f.Info()
			if err != nil {
				continue
			}
			title, cwd := claudePreview(path)
			if title == "" {
				title = "(sem prompt)"
			}
			out = append(out, Session{
				AgentID:   "claude-code",
				AgentName: "Claude Code",
				ID:        strings.TrimSuffix(f.Name(), ".jsonl"),
				Path:      path,
				CWD:       cwd,
				Title:     title,
				MTime:     info.ModTime(),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	return out, nil
}

// claudePreview lê as primeiras linhas do transcript e extrai o primeiro
// prompt real do usuário (ignora isMeta e tags de harness) e o cwd.
func claudePreview(path string) (title, cwd string) {
	for _, line := range firstLines(path, 50) {
		var e claudeLine
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		if cwd == "" && e.CWD != "" {
			cwd = e.CWD
		}
		if title == "" && e.Type == "user" && !e.IsMeta && e.Message.Role == "user" {
			title = cleanTitle(extractText(e.Message.Content), 80)
		}
		if title != "" && cwd != "" {
			break
		}
	}
	return title, cwd
}

// extractText lida com content string ou lista de blocos [{"type":"text",...}].
func extractText(content json.RawMessage) string {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

func (c *Claude) ResumeCmd(s Session) ([]string, string, bool) {
	dir := s.CWD
	if !dirExists(dir) {
		dir = c.Home
	}
	return []string{"claude", "--resume", s.ID}, dir, true
}

// ID implementa Adapter sem I/O.
func (c *Claude) ID() string { return "claude-code" }
