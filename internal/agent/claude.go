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

// Claude adapts Claude Code. Skills in ~/.claude/skills; sessions in
// ~/.claude/projects/<slug>/<sessionId>.jsonl.
type Claude struct {
	Home      string
	ConfigDir string                       // CLAUDE_CONFIG_DIR; empty means ~/.claude
	Look      func(string) (string, error) // injectable in tests
	// UsageURL overrides the subscription usage endpoint (tests).
	UsageURL string

	// ProviderState records which settings.json env keys lazyagents wrote, so
	// apply and clear never remove the user's own ("" = kept in memory).
	ProviderState string
	providerMem   *claudeProviderState

	// Index remembers what was read of each transcript (nil = in-memory, created lazily).
	Index     *Index
	indexOnce sync.Once

	// liveCache holds the JSONL paths some process has open, computed once per
	// ListSessions; IsLive only reads it, never runs lsof per session.
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

// liveWindow limits the open-file check to recently modified sessions: a live
// conversation writes to its transcript, and thousands of paths in one lsof
// cost hundreds of ms. Deleting checks the exact file at that moment.
const liveWindow = 24 * time.Hour

func NewClaude(home string) *Claude {
	return &Claude{Home: home, ConfigDir: envPath(home, "CLAUDE_CONFIG_DIR"), Look: exec.LookPath}
}

func (c *Claude) configDir() string {
	if c.ConfigDir != "" {
		return c.ConfigDir
	}
	return filepath.Join(c.Home, ".claude")
}
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
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", c.configDir())
	} else {
		a.Detail = DetailNotInstalled
	}
	return a
}

// claudeLine covers the Claude Code JSONL fields in use.
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
		return nil, nil // no projects = no sessions, not an error
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
			out[i].Title = "(no prompt)"
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

// IsLive reports whether this session's JSONL is open by some process (a live
// conversation). Reads the ListSessions cache: false before ListSessions.
func (c *Claude) IsLive(s Session) bool {
	c.liveMu.Lock()
	defer c.liveMu.Unlock()
	return c.liveCache[s.Path]
}

var (
	aiTitleKey = []byte(`"ai-title"`)
	usageKey   = []byte(`"usage"`)
)

// claudeIndexLine extracts what the index keeps from a JSONL line: the preview
// (preferred title is the LAST "ai-title" line, where Claude Code stores a
// rename; fallback is the first real user prompt, skipping isMeta and harness
// tags), token sums and responses for the usage module. Byte filters skip
// decoding what is irrelevant: tool output, most of the file, has no "usage".
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
	mu := l.Message.Usage
	u := Usage{
		Input:        mu.InputTokens,
		Output:       mu.OutputTokens,
		CacheRead:    mu.CacheReadInputTokens,
		CacheWrite:   mu.CacheCreationInputTokens,
		CacheWrite1h: min(mu.CacheCreation.Ephemeral1h, mu.CacheCreationInputTokens),
		Tier:         claudeTier(mu.Speed, mu.ServiceTier, mu.InferenceGeo),
	}
	e.HasUsage = true
	e.Usage.Input += u.Input
	e.Usage.Output += u.Output
	e.Usage.CacheRead += u.CacheRead
	e.Usage.CacheWrite += u.CacheWrite
	e.Usage.CacheWrite1h += u.CacheWrite1h
	if l.Message.Model != "" {
		e.Usage.Model = l.Message.Model
	}
	ts, _ := time.Parse(time.RFC3339, l.Timestamp) // no timestamp: zero, the session time applies
	e.addEvent(&e.Events, ts, l.Message.Model, l.CWD, u)
}

// extractText handles content as a string or a list of [{"type":"text",...}] blocks.
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

func (c *Claude) ID() string { return "claude-code" }

// Transcript links each Agent call to the subagent's own transcript:
// <session>/subagents/agent-<id>.jsonl, whose agent-<id>.meta.json names the
// parent's tool_use id ("toolUseId").
func (c *Claude) Transcript(s Session) ([]Entry, error) {
	subs := claudeSubagents(strings.TrimSuffix(s.Path, ".jsonl"))
	return jsonlTranscriptLinked(s.Path, func(id string, e *Entry) {
		if path, ok := subs[id]; ok {
			e.Sub = path
			if data, err := os.ReadFile(path); err == nil {
				e.Calls = bytes.Count(data, []byte(`"type":"tool_use"`))
			}
		}
	})
}

// claudeSubagents maps a session's tool_use ids to their subagent transcripts.
func claudeSubagents(sessionDir string) map[string]string {
	metas, _ := filepath.Glob(filepath.Join(sessionDir, "subagents", "agent-*.meta.json"))
	subs := map[string]string{}
	for _, meta := range metas {
		data, err := os.ReadFile(meta)
		if err != nil {
			continue
		}
		var m struct {
			ToolUseID string `json:"toolUseId"`
		}
		path := strings.TrimSuffix(meta, ".meta.json") + ".jsonl"
		if json.Unmarshal(data, &m) == nil && m.ToolUseID != "" {
			if _, err := os.Stat(path); err == nil {
				subs[m.ToolUseID] = path
			}
		}
	}
	return subs
}

// DeleteSession backs the JSONL up and removes it, checking the exact file for
// an open handle first (the live badge only looks at recent sessions).
func (c *Claude) DeleteSession(s Session, backupsDir string) error {
	if liveOpenFiles([]string{s.Path})[s.Path] {
		return fmt.Errorf("session in progress: close it before deleting")
	}
	return deleteSessionFile(s.Path, backupsDir)
}

// assistantUsageLine covers only the usage fields of assistant lines (same shape
// as the Anthropic Messages API).
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
			CacheCreation            struct {
				Ephemeral1h int `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
			ServiceTier  string `json:"service_tier"`
			Speed        string `json:"speed"`
			InferenceGeo string `json:"inference_geo"`
		} `json:"usage"`
	} `json:"message"`
}

// claudeTier is the pricing tier a response ran under, from its usage record;
// standard values give "". Unknown ones are kept so the price stays unknown.
func claudeTier(speed, serviceTier, geo string) string {
	var t []string
	if speed != "" && speed != "standard" {
		t = append(t, speed) // "fast"
	}
	if serviceTier != "" && serviceTier != "standard" {
		t = append(t, serviceTier) // "batch", "priority"
	}
	if geo != "" && geo != "global" && geo != "not_available" {
		t = append(t, "geo:"+geo)
	}
	return strings.Join(t, ",")
}

// SessionUsage sums usage over all assistant lines. Best-effort: unreadable
// lines are skipped; no usage at all gives ok=false.
func (c *Claude) SessionUsage(s Session) (Usage, bool) {
	e, err := c.index().refresh(s.Path, claudeIndexLine)
	if err != nil || !e.HasUsage {
		return Usage{}, false
	}
	return e.Usage, true
}

// UsageEvents returns assistant responses, bucketed by the index, with the
// JSONL timestamps.
func (c *Claude) UsageEvents(s Session) ([]UsageEvent, error) {
	e, err := c.index().refresh(s.Path, claudeIndexLine)
	if err != nil {
		return nil, fmt.Errorf("reading session %s: %w", s.ID, err)
	}
	return e.events(e.Events, s), nil
}

// claudeCreds covers only the non-secret part of ~/.claude/.credentials.json:
// subscription type and tier. Tokens are a secret (presence only).
type claudeCreds struct {
	OAuth struct {
		SubscriptionType string `json:"subscriptionType"`
		RateLimitTier    string `json:"rateLimitTier"`
		AccessToken      secret `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// AuthMode: OAuth credentials = subscription; otherwise an API key in the env.
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
