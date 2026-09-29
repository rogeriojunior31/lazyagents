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

func (p *Pi) expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		return filepath.Join(p.Home, path[1:])
	}
	return path
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
)

// piLine is the part of a session line the index and transcript need.
type piLine struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	ParentID *string         `json:"parentId"`
	CWD      string          `json:"cwd"`  // session header
	Name     string          `json:"name"` // session_info: the /name title
	Message  json.RawMessage `json:"message"`
}

func piIndexLine(e *indexEntry, line []byte) {
	infoLine := bytes.Contains(line, piSessionInfo)
	promptLine := e.FirstPrompt == "" && bytes.Contains(line, piUserRole)
	if e.CWD != "" && !infoLine && !promptLine {
		return
	}
	var l piLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	switch l.Type {
	case "session":
		e.CWD = l.CWD
	case "session_info": // the last rename wins
		e.AITitle = l.Name
	case "message":
		if promptLine {
			e.FirstPrompt = cleanTitle(looseUserText(line), 80)
		}
	}
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
