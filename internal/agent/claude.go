package agent

import (
	"bufio"
	"bytes"
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

	// liveCache é o conjunto de paths de JSONL abertos por algum processo
	// agora, calculado uma vez por ListSessions (M8.A2) — IsLive só consulta
	// o cache, nunca chama lsof por sessão.
	liveCache map[string]bool
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
	AITitle string `json:"aiTitle"`
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
	paths := make([]string, len(out))
	for i, s := range out {
		paths[i] = s.Path
	}
	c.liveCache = liveOpenFiles(paths)
	return out, nil
}

// liveOpenFiles devolve, dentre paths, os que estão abertos por algum
// processo agora (via lsof, um único processo para todos de uma vez — nunca
// um lsof por sessão). Best-effort: sem lsof no PATH ou nenhum aberto, mapa
// vazio, nunca erro.
func liveOpenFiles(paths []string) map[string]bool {
	live := make(map[string]bool)
	if len(paths) == 0 {
		return live
	}
	out, _ := exec.Command("lsof", append([]string{"--"}, paths...)...).Output()
	for _, p := range paths {
		if strings.Contains(string(out), p) {
			live[p] = true
		}
	}
	return live
}

// IsLive diz se o JSONL desta sessão está aberto por algum processo agora —
// sinal de conversa em andamento (M8.A2). Consulta o cache de ListSessions;
// chamar antes de ListSessions sempre devolve false.
func (c *Claude) IsLive(s Session) bool {
	return c.liveCache[s.Path]
}

// claudePreview varre o transcript e extrai o título e o cwd da sessão.
// O título preferido é a ÚLTIMA linha "ai-title" (é onde o Claude Code guarda
// o nome dado via rename); fallback é o primeiro prompt real do usuário
// (ignorando isMeta e tags de harness).
func claudePreview(path string) (title, cwd string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var firstPrompt, aiTitle string
	for sc.Scan() {
		line := sc.Bytes()
		// filtro barato antes do unmarshal: só interessam 3 tipos de linha
		wantTitle := bytes.Contains(line, []byte(`"ai-title"`))
		wantMore := cwd == "" || firstPrompt == ""
		if !wantTitle && !wantMore {
			continue
		}
		var e claudeLine
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		if cwd == "" && e.CWD != "" {
			cwd = e.CWD
		}
		if e.Type == "ai-title" && e.AITitle != "" {
			aiTitle = e.AITitle
		}
		if firstPrompt == "" && e.Type == "user" && !e.IsMeta && e.Message.Role == "user" {
			firstPrompt = cleanTitle(extractText(e.Message.Content), 80)
		}
	}
	if aiTitle != "" {
		return cleanTitle(aiTitle, 80), cwd
	}
	return firstPrompt, cwd
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

// Transcript lê as mensagens do JSONL da sessão.
func (c *Claude) Transcript(s Session) ([]Entry, error) {
	return jsonlTranscript(s.Path)
}

// DeleteSession faz backup do JSONL da sessão e remove o original.
func (c *Claude) DeleteSession(s Session, backupsDir string) error {
	return deleteSessionFile(s.Path, backupsDir)
}

// assistantUsageLine cobre só os campos de usage das linhas assistant do
// JSONL — mesmo formato da API de mensagens da Anthropic.
type assistantUsageLine struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// SessionUsage soma o usage de todas as linhas assistant do JSONL. Best-effort:
// linha ilegível é pulada; sem nenhuma linha com usage, ok=false.
func (c *Claude) SessionUsage(s Session) (Usage, bool) {
	f, err := os.Open(s.Path)
	if err != nil {
		return Usage{}, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var u Usage
	found := false
	for sc.Scan() {
		var e assistantUsageLine
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Type != "assistant" || e.Message.Usage == nil {
			continue
		}
		found = true
		u.Input += e.Message.Usage.InputTokens
		u.Output += e.Message.Usage.OutputTokens
		u.CacheRead += e.Message.Usage.CacheReadInputTokens
		u.CacheWrite += e.Message.Usage.CacheCreationInputTokens
		if e.Message.Model != "" {
			u.Model = e.Message.Model
		}
	}
	return u, found
}
