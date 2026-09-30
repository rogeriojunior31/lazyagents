package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// OpenCode adapts opencode. Skills in ~/.config/opencode/skills (it also reads
// ~/.claude/skills and ~/.agents/skills); sessions in a SQLite db read through
// the sqlite3 binary (no cgo).
type OpenCode struct {
	Home string
	Look func(string) (string, error)
}

func NewOpenCode(home string) *OpenCode { return &OpenCode{Home: home, Look: exec.LookPath} }

func (o *OpenCode) configDir() string { return filepath.Join(o.Home, ".config", "opencode") }
func (o *OpenCode) dbPath() string {
	return filepath.Join(o.Home, ".local", "share", "opencode", "opencode.db")
}

func (o *OpenCode) Detect() Agent {
	bin, _ := o.Look("opencode")
	a := Agent{
		ID:         "opencode",
		Name:       "OpenCode",
		Short:      "O",
		Installed:  bin != "" || dirExists(o.configDir()),
		ManagedDir: filepath.Join(o.configDir(), "skills"),
	}
	a.SharedDir = filepath.Join(o.Home, ".agents", "skills")
	a.ReadDirs = []string{a.ManagedDir, filepath.Join(o.Home, ".claude", "skills"), a.SharedDir}
	if bin != "" {
		a.Version = version(bin)
		a.Detail = bin
	} else if a.Installed {
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", o.configDir())
	} else {
		a.Detail = DetailNotInstalled
	}
	return a
}

type opencodeRow struct {
	ID        string `json:"id"`
	Directory string `json:"directory"`
	Title     string `json:"title"`
	Updated   int64  `json:"time_updated"`
}

func (o *OpenCode) ListSessions() ([]Session, error) {
	db := o.dbPath()
	if _, err := os.Stat(db); err != nil {
		return nil, nil
	}
	sqlite, err := o.Look("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("opencode sessions live in SQLite: install sqlite3 to list them")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const q = `SELECT id, directory, title, time_updated FROM session
		WHERE parent_id IS NULL ORDER BY time_updated DESC LIMIT 500`
	out, err := exec.CommandContext(ctx, sqlite, "-json", "-readonly", db, q).Output()
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", db, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	var rows []opencodeRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("reading sqlite3 output: %w", err)
	}
	sessions := make([]Session, 0, len(rows))
	for _, r := range rows {
		title := cleanTitle(r.Title, 80)
		if title == "" {
			title = "(untitled)"
		}
		sessions = append(sessions, Session{
			AgentID:   "opencode",
			AgentName: "OpenCode",
			ID:        r.ID,
			Path:      db,
			CWD:       r.Directory,
			Title:     title,
			MTime:     time.UnixMilli(r.Updated),
		})
	}
	return sessions, nil
}

func (o *OpenCode) ResumeCmd(s Session) ([]string, string, bool) {
	// original CWD, no fallback: callers decide what to do with a bad dir
	return []string{"opencode", "--session", s.ID}, s.CWD, true
}

func (o *OpenCode) ID() string { return "opencode" }

// Transcript queries the session messages with sqlite3; each message's data
// column is JSON, parsed leniently.
func (o *OpenCode) Transcript(s Session) ([]Entry, error) {
	sqlite, err := o.Look("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("the opencode transcript needs the sqlite3 binary")
	}
	id := strings.ReplaceAll(s.ID, "'", "''")
	// Current opencode keeps a message's content in the part table and only
	// metadata (role, model, tokens) in message.data.
	var parts []struct {
		Msg  string `json:"msg"`
		Part string `json:"part"`
	}
	q := fmt.Sprintf(`SELECT m.data AS msg, p.data AS part FROM part p JOIN message m ON m.id = p.message_id
		WHERE p.session_id='%s' ORDER BY m.time_created, m.id, p.time_created, p.id LIMIT %d`, id, maxTranscriptEntries*4)
	partsErr := o.queryJSON(sqlite, q, &parts)
	if partsErr == nil && len(parts) > 0 {
		var entries []Entry
		for _, r := range parts {
			entries = append(entries, openCodePart(r.Msg, r.Part)...)
		}
		entries = mergeTexts(entries)
		return entries[:min(len(entries), maxTranscriptEntries)], nil
	}
	// Older opencode kept the whole message, parts included, in message.data.
	var rows []struct {
		Data string `json:"data"`
	}
	q = fmt.Sprintf(`SELECT data FROM message WHERE session_id='%s'
		ORDER BY time_created ASC LIMIT %d`, id, maxTranscriptEntries)
	if err := o.queryJSON(sqlite, q, &rows); err != nil {
		return nil, fmt.Errorf("querying messages of %s: %w", s.ID, errors.Join(partsErr, err))
	}
	var entries []Entry
	for _, r := range rows {
		if e, ok := entryFromLine([]byte(r.Data)); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// queryJSON runs a read-only query with `sqlite3 -json`; no row is an empty result.
func (o *OpenCode) queryJSON(sqlite, q string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, sqlite, "-json", "-readonly", o.dbPath(), q).Output()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("reading sqlite3 output: %w", err)
	}
	return nil
}

// openCodePart turns one part of a message into transcript entries: text,
// reasoning and tool calls; step markers, snapshots and injected (synthetic)
// text are not conversation. A prompt starting with "<" is the user's own.
func openCodePart(msgData, partData string) []Entry {
	var msg struct {
		Role string `json:"role"`
	}
	var part struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		Synthetic bool   `json:"synthetic"`
		Tool      string `json:"tool"`
		State     struct {
			Input map[string]any `json:"input"`
		} `json:"state"`
	}
	if json.Unmarshal([]byte(msgData), &msg) != nil || json.Unmarshal([]byte(partData), &part) != nil {
		return nil
	}
	role := RoleUser
	if msg.Role == "assistant" {
		role = RoleAssistant
	}
	switch part.Type {
	case "text":
		text := strings.TrimSpace(part.Text)
		if part.Synthetic || text == "" { // injected text is marked synthetic
			return nil
		}
		return []Entry{{Role: role, Text: capRunes(text)}}
	case "reasoning":
		if role == RoleAssistant {
			return thinkingEntry([]string{part.Text})
		}
	case "tool":
		if e, ok := toolEntry(map[string]any{"name": part.Tool, "input": part.State.Input}); ok && role == RoleAssistant {
			return []Entry{e}
		}
	}
	return nil
}

// DeleteSession delegates to the opencode CLI: sessions live in SQLite and
// cannot be removed by moving a file. `opencode export` is the backup; without
// a valid export nothing is deleted.
func (o *OpenCode) DeleteSession(s Session, backupsDir string) error {
	bin, err := o.Look("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH: %w", err)
	}
	if s.ID == "" || s.ID != filepath.Base(s.ID) || strings.HasPrefix(s.ID, ".") || strings.ContainsAny(s.ID, `/\`) {
		return fmt.Errorf("opencode session id %q is not a safe file name", s.ID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "export", s.ID)
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("opencode export (backup before delete): %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if !json.Valid(bytes.TrimSpace(data)) {
		return fmt.Errorf("opencode export (backup before delete): output is not JSON")
	}
	backup := filepath.Join(backupsDir, fmt.Sprintf("opencode-%s.%s.json", s.ID, time.Now().Format("20060102T150405.000000000")))
	if err := fsutil.WriteAtomic(backup, data, 0o600); err != nil {
		return fmt.Errorf("backing up opencode session: %w", err)
	}
	out, err := exec.CommandContext(ctx, bin, "session", "delete", s.ID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("opencode session delete: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
