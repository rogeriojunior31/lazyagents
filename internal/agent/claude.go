package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Claude adapta o Claude Code (CLI). Skills em ~/.claude/skills; sessões em
// ~/.claude/projects/<slug>/<sessionId>.jsonl.
type Claude struct {
	Home string
	Look func(string) (string, error) // injetável em teste
	// UsageURL substitui o endpoint de uso da assinatura (injetável em teste).
	UsageURL string

	// Index guarda o que já foi lido de cada transcript (nil = um índice só
	// em memória, criado no primeiro uso).
	Index     *Index
	indexOnce sync.Once

	// liveCache é o conjunto de paths de JSONL abertos por algum processo
	// agora, calculado uma vez por ListSessions — IsLive só consulta
	// o cache, nunca chama lsof por sessão.
	liveCache map[string]bool
	liveMu    sync.Mutex
}

func (c *Claude) index() *Index {
	c.indexOnce.Do(func() {
		if c.Index == nil {
			c.Index = NewIndex("")
		}
	})
	return c.Index
}

// liveWindow limita o lsof às sessões modificadas há pouco: conversa em
// andamento escreve no transcript, e milhares de caminhos num lsof custam
// centenas de milissegundos. Deletar confere o arquivo exato na hora.
const liveWindow = 24 * time.Hour

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
			info, err := f.Info()
			if err != nil {
				continue
			}
			out = append(out, Session{
				AgentID:   "claude-code",
				AgentName: "Claude Code",
				ID:        strings.TrimSuffix(f.Name(), ".jsonl"),
				Path:      filepath.Join(projDir, f.Name()),
				MTime:     info.ModTime(),
			})
		}
	}
	idx := c.index()
	paths := make([]string, len(out))
	keep := make(map[string]bool, len(out))
	for i, s := range out {
		paths[i], keep[s.Path] = s.Path, true
	}
	idx.refreshAll(paths, claudeIndexLine)
	for i := range out {
		e, _ := idx.get(out[i].Path)
		out[i].CWD, out[i].Title = e.CWD, e.FirstPrompt
		if e.AITitle != "" {
			out[i].Title = cleanTitle(e.AITitle, 80)
		}
		if out[i].Title == "" {
			out[i].Title = "(sem prompt)"
		}
	}
	idx.retain(c.projectsDir()+string(filepath.Separator), keep)
	idx.save()

	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	var recent []string
	cut := time.Now().Add(-liveWindow)
	for _, s := range out {
		if s.MTime.After(cut) {
			recent = append(recent, s.Path)
		}
	}
	live := liveOpenFiles(recent)
	c.liveMu.Lock()
	c.liveCache = live
	c.liveMu.Unlock()
	return out, nil
}

// IsLive diz se o JSONL desta sessão está aberto por algum processo agora —
// sinal de conversa em andamento. Consulta o cache de ListSessions;
// chamar antes de ListSessions sempre devolve false.
func (c *Claude) IsLive(s Session) bool {
	c.liveMu.Lock()
	defer c.liveMu.Unlock()
	return c.liveCache[s.Path]
}

var (
	aiTitleKey = []byte(`"ai-title"`)
	usageKey   = []byte(`"usage"`)
)

// claudeIndexLine extrai de uma linha do JSONL tudo que o índice guarda:
// prévia (o título preferido é a ÚLTIMA linha "ai-title", onde o Claude Code
// guarda o nome dado via rename; fallback é o primeiro prompt real do
// usuário, ignorando isMeta e tags de harness), soma de tokens e as respostas
// para o módulo de uso. Filtros de bytes evitam decodificar o que não
// interessa: saída de ferramenta, a maior parte do arquivo, não tem "usage".
func claudeIndexLine(e *indexEntry, line []byte) {
	if bytes.Contains(line, aiTitleKey) {
		var l claudeLine
		if json.Unmarshal(line, &l) == nil && l.Type == "ai-title" && l.AITitle != "" {
			e.AITitle = l.AITitle
		}
	}
	if e.CWD == "" || e.FirstPrompt == "" {
		var l claudeLine
		if json.Unmarshal(line, &l) == nil {
			if e.CWD == "" && l.CWD != "" {
				e.CWD = l.CWD
			}
			if e.FirstPrompt == "" && l.Type == "user" && !l.IsMeta && l.Message.Role == "user" {
				e.FirstPrompt = cleanTitle(extractText(l.Message.Content), 80)
			}
		}
	}
	if !bytes.Contains(line, usageKey) {
		return
	}
	var l assistantUsageLine
	if json.Unmarshal(line, &l) != nil || l.Type != "assistant" || l.Message.Usage == nil {
		return
	}
	u := Usage{
		Input:      l.Message.Usage.InputTokens,
		Output:     l.Message.Usage.OutputTokens,
		CacheRead:  l.Message.Usage.CacheReadInputTokens,
		CacheWrite: l.Message.Usage.CacheCreationInputTokens,
	}
	e.HasUsage = true
	e.Usage.Input += u.Input
	e.Usage.Output += u.Output
	e.Usage.CacheRead += u.CacheRead
	e.Usage.CacheWrite += u.CacheWrite
	if l.Message.Model != "" {
		e.Usage.Model = l.Message.Model
	}
	ts, _ := time.Parse(time.RFC3339, l.Timestamp) // sem data: zero, a da sessão vale
	e.addEvent(&e.Events, ts, l.Message.Model, l.CWD, u)
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

// DeleteSession faz backup do JSONL da sessão e remove o original. Confere
// com lsof o arquivo exato na hora: o badge de viva só olha as recentes.
func (c *Claude) DeleteSession(s Session, backupsDir string) error {
	if liveOpenFiles([]string{s.Path})[s.Path] {
		return fmt.Errorf("sessão em andamento — feche-a antes de deletar")
	}
	return deleteSessionFile(s.Path, backupsDir)
}

// assistantUsageLine cobre só os campos de usage das linhas assistant do
// JSONL — mesmo formato da API de mensagens da Anthropic.
type assistantUsageLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
	Message   struct {
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
	e, err := c.index().refresh(s.Path, claudeIndexLine)
	if err != nil || !e.HasUsage {
		return Usage{}, false
	}
	return e.Usage, true
}

// UsageEvents devolve as respostas do assistente, agrupadas pelo índice em
// faixas de 5 min, com a data das linhas do JSONL.
func (c *Claude) UsageEvents(s Session) ([]UsageEvent, error) {
	e, err := c.index().refresh(s.Path, claudeIndexLine)
	if err != nil {
		return nil, fmt.Errorf("lendo sessão %s: %w", s.ID, err)
	}
	return e.events(e.Events, s), nil
}

// claudeCreds cobre só o que não é segredo em ~/.claude/.credentials.json:
// o tipo de assinatura e o tier. Os tokens entram como secret (presença).
type claudeCreds struct {
	OAuth struct {
		SubscriptionType string `json:"subscriptionType"`
		RateLimitTier    string `json:"rateLimitTier"`
		AccessToken      secret `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// AuthMode: credenciais OAuth = assinatura; senão, chave de API no ambiente.
func (c *Claude) AuthMode() (AuthMode, string) {
	var creds claudeCreds
	if err := decodeJSONFile(filepath.Join(c.configDir(), ".credentials.json"), &creds); err == nil {
		if t := creds.OAuth.SubscriptionType; t != "" {
			detail := t
			if tier := creds.OAuth.RateLimitTier; tier != "" && tier != t {
				detail += " · " + tier
			}
			return AuthSubscription, detail
		}
		if bool(creds.OAuth.AccessToken) {
			return AuthSubscription, ""
		}
	}
	for _, env := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		if os.Getenv(env) != "" {
			return AuthAPIKey, env
		}
	}
	return AuthUnknown, ""
}
