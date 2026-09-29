package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Pi adapts the pi coding agent (earendil-works/pi). Everything lives in one
// agent dir, ~/.pi/agent unless PI_CODING_AGENT_DIR points elsewhere; sessions
// in <agent dir>/sessions/--<cwd>--/<timestamp>_<id>.jsonl.
type Pi struct {
	Home       string
	Dir        string // PI_CODING_AGENT_DIR; empty means ~/.pi/agent
	SessionDir string // PI_CODING_AGENT_SESSION_DIR; empty means settings.json or the default
	Look       func(string) (string, error)
	// Index remembers what was read of each session (nil = memory only).
	Index     *Index
	indexOnce sync.Once
	// ProviderState keeps the user's default provider and model a profile
	// replaced ("" = memory only, for tests).
	ProviderState string
	providerMem   *piProviderState
}

func NewPi(home string) *Pi {
	return &Pi{Home: home, Dir: os.Getenv("PI_CODING_AGENT_DIR"),
		SessionDir: os.Getenv("PI_CODING_AGENT_SESSION_DIR"), Look: exec.LookPath}
}

func (p *Pi) ID() string { return "pi" }

func (p *Pi) index() *Index {
	p.indexOnce.Do(func() {
		if p.Index == nil {
			p.Index = NewIndex("")
		}
	})
	return p.Index
}

// expand resolves a leading ~ and cleans the path: the index prunes deleted
// sessions by path prefix, which a trailing slash would break.
func (p *Pi) expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		return filepath.Join(p.Home, path[1:])
	}
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func (p *Pi) agentDir() string {
	if p.Dir == "" {
		return filepath.Join(p.Home, ".pi", "agent")
	}
	return p.expand(p.Dir)
}

// sessionsDir follows pi's precedence: the env var, then sessionDir in
// settings.json. A relative sessionDir resolves from each project's cwd, so
// there is no single dir to list: the default is used then.
func (p *Pi) sessionsDir() string {
	if p.SessionDir != "" {
		return p.expand(p.SessionDir)
	}
	var settings struct {
		SessionDir string `json:"sessionDir"`
	}
	if decodeJSONFile(filepath.Join(p.agentDir(), "settings.json"), &settings) == nil {
		if dir := p.expand(settings.SessionDir); filepath.IsAbs(dir) {
			return dir
		}
	}
	return filepath.Join(p.agentDir(), "sessions")
}

// semver tells pi apart from other tools that ship a binary named `pi`.
var semver = regexp.MustCompile(`^v?\d+\.\d+\.\d+\S*$`)

func (p *Pi) Detect() Agent {
	a := Agent{ID: "pi", Name: "Pi", Short: "P"}
	dir := p.agentDir()
	hasDir := dirExists(dir)
	bin, _ := p.Look("pi")
	if bin != "" {
		a.Version = version(bin)
		if !hasDir && !semver.MatchString(a.Version) {
			bin, a.Version = "", ""
		}
	}
	a.Installed = bin != "" || hasDir
	if !a.Installed {
		a.Detail = DetailNotInstalled
		return a
	}
	a.ManagedDir = filepath.Join(dir, "skills")
	a.SharedDir = filepath.Join(p.Home, ".agents", "skills")
	a.ReadDirs = []string{a.ManagedDir, a.SharedDir}
	if bin != "" {
		a.Detail = bin
	} else {
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", dir)
	}
	return a
}

func (p *Pi) ListSessions() ([]Session, error) {
	root := p.sessionsDir()
	projects, err := os.ReadDir(root)
	if err != nil {
		return nil, nil // no sessions dir = no sessions, not an error
	}
	var out []Session
	for _, proj := range projects {
		if !proj.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, proj.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			name := f.Name()
			info, err := f.Info()
			if f.IsDir() || !strings.HasSuffix(name, ".jsonl") || err != nil {
				continue
			}
			// <timestamp>_<id>.jsonl, the id URI-encoded
			id := strings.TrimSuffix(name, ".jsonl")
			if _, after, ok := strings.Cut(id, "_"); ok {
				id = after
			}
			if dec, err := url.PathUnescape(id); err == nil {
				id = dec
			}
			out = append(out, Session{AgentID: "pi", AgentName: "Pi", ID: id,
				Path: filepath.Join(root, proj.Name(), name), MTime: info.ModTime()})
		}
	}
	idx := p.index()
	paths := make([]string, len(out))
	keep := make(map[string]bool, len(out))
	for i, s := range out {
		paths[i], keep[s.Path] = s.Path, true
	}
	idx.refreshAll(paths, piIndexLine)
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
	idx.retain(root+string(filepath.Separator), keep)
	idx.save()
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	return out, nil
}

var (
	piSessionInfo = []byte(`"type":"session_info"`)
	piUserRole    = []byte(`"role":"user"`)
	piUsageKey    = []byte(`"usage":`)
	piModelChange = []byte(`"type":"model_change"`)
)

// piLine is the part of a session line the index and transcript need.
type piLine struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	ParentID  *string         `json:"parentId"`
	Timestamp string          `json:"timestamp"`
	CWD       string          `json:"cwd"`   // session header
	Name      string          `json:"name"`  // session_info: the /name title
	Model     string          `json:"model"` // usage entry
	Provider  string          `json:"provider"`
	ModelID   string          `json:"modelId"`
	Usage     *piUsage        `json:"usage"` // usage, compaction and branch_summary entries
	Message   json.RawMessage `json:"message"`
}

type piMessage struct {
	Role     string   `json:"role"`
	Model    string   `json:"model"`
	Provider string   `json:"provider"`
	Usage    *piUsage `json:"usage"` // assistant; toolResult for nested model work
}

// piModelKey keeps the provider with the model in the index: whether a call
// cost money depends on how that provider is signed in, decided at read time.
const piModelSep = "\x1f"

func piModelKey(provider, model string) string { return provider + piModelSep + model }

func piSplitModel(key string) (provider, model string) {
	if p, m, ok := strings.Cut(key, piModelSep); ok {
		return p, m
	}
	return "", key
}

// piUsage is pi's per-call usage; reasoning is already inside output, and cost
// is pi's own USD figure from its model catalog.
type piUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
	Cost       struct {
		Total float64 `json:"total"`
	} `json:"cost"`
}

func piIndexLine(e *indexEntry, line []byte) {
	infoLine := bytes.Contains(line, piSessionInfo)
	promptLine := e.FirstPrompt == "" && bytes.Contains(line, piUserRole)
	usageLine := bytes.Contains(line, piUsageKey)
	if e.CWD != "" && !infoLine && !promptLine && !usageLine && !bytes.Contains(line, piModelChange) {
		return
	}
	var l piLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	u, model, provider := l.Usage, l.Model, l.Provider
	switch l.Type {
	case "session":
		e.CWD = l.CWD
	case "session_info": // the last rename wins
		e.AITitle = l.Name
	case "model_change":
		e.Model = piModelKey(l.Provider, l.ModelID)
	case "message":
		if promptLine {
			e.FirstPrompt = cleanTitle(looseUserText(line), 80)
		}
		var m piMessage
		if usageLine && json.Unmarshal(l.Message, &m) == nil {
			u, model, provider = m.Usage, m.Model, m.Provider
		}
	}
	if u == nil {
		return
	}
	key := piModelKey(provider, model)
	if model == "" {
		key = e.Model // compaction and branch summaries run on the session model
	} else {
		e.Model = key
	}
	usage := Usage{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Cost: u.Cost.Total}
	e.HasUsage = true
	ts, _ := time.Parse(time.RFC3339, l.Timestamp) // no timestamp: zero, the session time applies
	e.addEvent(&e.Events, ts, key, e.CWD, usage)
}

// SessionUsage sums the session's events, so calls covered by a subscription
// count their tokens but not their cost; the cost is known only when every
// call's is.
func (p *Pi) SessionUsage(s Session) (Usage, bool) {
	events, err := p.UsageEvents(s)
	if err != nil || len(events) == 0 {
		return Usage{}, false
	}
	sum := Usage{CostKnown: true}
	for _, ev := range events {
		sum.Input += ev.Usage.Input
		sum.Output += ev.Usage.Output
		sum.CacheRead += ev.Usage.CacheRead
		sum.CacheWrite += ev.Usage.CacheWrite
		sum.Cost += ev.Usage.Cost
		sum.CostKnown = sum.CostKnown && ev.Usage.CostKnown
		sum.Model = ev.Model
	}
	return sum, true
}

// UsageEvents gives each call pi's recorded cost, which is authoritative even
// when 0 (pi prices every call from its catalog), except calls to a provider
// signed in with OAuth: a subscription does not pay per token.
func (p *Pi) UsageEvents(s Session) ([]UsageEvent, error) {
	e, err := p.index().refresh(s.Path, piIndexLine)
	if err != nil {
		return nil, fmt.Errorf("reading session %s: %w", s.ID, err)
	}
	creds := p.credentialTypes()
	events := e.events(e.Events, s)
	for i := range events {
		provider, model := piSplitModel(events[i].Model)
		events[i].Model, events[i].Usage.Model = model, model
		events[i].Usage.CostKnown = true
		if creds[provider] == "oauth" {
			events[i].Usage.Cost = 0
		}
	}
	return events, nil
}

// credentialTypes maps each provider with a stored credential to its type:
// "oauth" (a subscription login) or "api_key", from auth.json and the apiKey
// of models.json. Only the type is read, never the value.
func (p *Pi) credentialTypes() map[string]string {
	types := map[string]string{}
	var models struct {
		Providers map[string]struct {
			APIKey piKey `json:"apiKey"`
		} `json:"providers"`
	}
	if decodeJSONFile(filepath.Join(p.agentDir(), "models.json"), &models) == nil {
		for id, pr := range models.Providers {
			if pr.APIKey.has { // piKey drops lazyagents' own placeholder
				types[id] = "api_key"
			}
		}
	}
	var auth map[string]struct {
		Type string `json:"type"`
	}
	if decodeJSONFile(filepath.Join(p.agentDir(), "auth.json"), &auth) == nil {
		for id, a := range auth {
			if a.Type != "" {
				types[id] = a.Type // auth.json wins, as in pi
			}
		}
	}
	return types
}

// AuthMode is API key when any provider pi can use is billed per token, so
// the cost column shows; each call is then priced by its own provider
// (UsageEvents). detail is the default provider.
func (p *Pi) AuthMode() (AuthMode, string) {
	var settings struct {
		DefaultProvider string `json:"defaultProvider"`
	}
	_ = decodeJSONFile(filepath.Join(p.agentDir(), "settings.json"), &settings)
	mode := AuthUnknown
	for _, t := range p.credentialTypes() {
		switch {
		case t == "api_key":
			return AuthAPIKey, settings.DefaultProvider
		case t == "oauth":
			mode = AuthSubscription
		}
	}
	return mode, settings.DefaultProvider
}

func (p *Pi) ResumeCmd(s Session) ([]string, string, bool) {
	dir := s.CWD
	if !dirExists(dir) {
		dir = p.Home
	}
	return []string{"pi", "--session", s.Path}, dir, true
}

// Transcript follows the active branch only: pi keeps every branch of the
// conversation tree in one file, and the last entry written is the current leaf.
func (p *Pi) Transcript(s Session) ([]Entry, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	byID := map[string]piLine{}
	var all []piLine
	var leaf string
	for line := range bytes.Lines(data) {
		var l piLine
		if json.Unmarshal(line, &l) != nil || l.Type == "session" {
			continue
		}
		all = append(all, l)
		if l.ID != "" {
			byID[l.ID] = l
			leaf = l.ID
		}
	}
	branch := all // version 1 sessions have no tree (no ids): read straight through
	if leaf != "" {
		branch = nil
	}
	for id := leaf; id != ""; {
		l, ok := byID[id]
		if !ok {
			break
		}
		delete(byID, id) // a parent cycle in a damaged file ends the walk
		branch = append(branch, l)
		id = ""
		if l.ParentID != nil {
			id = *l.ParentID
		}
	}
	if leaf != "" {
		slices.Reverse(branch)
	}
	var out []Entry
	for _, l := range branch {
		if l.Type == "message" && len(out) < maxTranscriptEntries {
			out = append(out, entriesFromLine(l.Message)...)
		}
	}
	return out, nil
}

func (p *Pi) DeleteSession(s Session, backupsDir string) error {
	return deleteSessionFile(s.Path, backupsDir)
}
