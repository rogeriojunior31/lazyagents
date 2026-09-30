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
	"sort"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Crush keeps sessions per project: projects.json in its data dir lists each
// project and its data dir, whose crush.db (SQLite) holds the sessions. It is
// read with the sqlite3 binary, read-only, like OpenCode's.

type crushProject struct {
	Path    string `json:"path"`
	DataDir string `json:"data_dir"`
}

func (c *Crush) projects() []crushProject {
	var idx struct {
		Projects []crushProject `json:"projects"`
	}
	if decodeJSONFile(filepath.Join(c.dataDir(), "projects.json"), &idx) != nil {
		return nil
	}
	return idx.Projects
}

// crushQuery runs a read-only query on one project database.
func (c *Crush) crushQuery(db, q string, out any) error {
	sqlite, err := c.Look("sqlite3")
	if err != nil {
		return errors.New("crush sessions live in SQLite: install sqlite3 to list them")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, sqlite, "-json", "-readonly", db, q).Output()
	if err != nil {
		return fmt.Errorf("querying %s: %w", db, err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// ListSessions lists each project's top-level sessions; sub-agent sessions
// (parent_session_id set) belong to the one that started them.
func (c *Crush) ListSessions() ([]Session, error) {
	var out []Session
	var errs []error
	for _, p := range c.projects() {
		db := filepath.Join(p.DataDir, "crush.db")
		if _, err := os.Stat(db); err != nil {
			continue
		}
		var rows []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			UpdatedAt int64  `json:"updated_at"`
		}
		q := `SELECT id, title, updated_at FROM sessions WHERE parent_session_id IS NULL OR parent_session_id = ''`
		if err := c.crushQuery(db, q, &rows); err != nil {
			errs = append(errs, err)
			continue
		}
		for _, r := range rows {
			title := cleanTitle(r.Title, 80)
			if title == "" {
				title = "(untitled)"
			}
			out = append(out, Session{AgentID: "crush", AgentName: "Crush", ID: r.ID, Path: db,
				CWD: p.Path, Title: title, MTime: time.Unix(r.UpdatedAt, 0)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

func (c *Crush) ResumeCmd(s Session) ([]string, string, bool) {
	dir := s.CWD
	if !dirExists(dir) {
		dir = c.Home
	}
	return []string{"crush", "--session", s.ID}, dir, true // run in dir: Crush finds sessions from it
}

// Transcript reads the session's messages in order; each holds a JSON list of
// parts: text, reasoning, tool calls and results (results are left out).
func (c *Crush) Transcript(s Session) ([]Entry, error) {
	var rows []struct {
		Role  string `json:"role"`
		Parts string `json:"parts"`
	}
	id := strings.ReplaceAll(s.ID, "'", "''")
	q := fmt.Sprintf(`SELECT role, parts FROM messages WHERE session_id='%s' ORDER BY created_at, rowid LIMIT %d`, id, maxTranscriptEntries)
	if err := c.crushQuery(s.Path, q, &rows); err != nil {
		return nil, err
	}
	var out []Entry
	for _, r := range rows {
		out = append(out, crushParts(r.Role, r.Parts)...)
	}
	return mergeTexts(out), nil
}

func crushParts(role, raw string) []Entry {
	var parts []struct {
		Type string `json:"type"`
		Data struct {
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
			Name     string `json:"name"`
			Input    string `json:"input"` // a JSON string
		} `json:"data"`
	}
	if json.Unmarshal([]byte(raw), &parts) != nil {
		return nil
	}
	entryRole := RoleUser
	switch role {
	case "assistant":
		entryRole = RoleAssistant
	case "user":
	default:
		return nil // tool results and system messages are not conversation
	}
	var out []Entry
	for _, p := range parts {
		switch p.Type {
		case "text":
			if t := strings.TrimSpace(p.Data.Text); t != "" {
				out = append(out, Entry{Role: entryRole, Text: capRunes(t)})
			}
		case "reasoning":
			if entryRole == RoleAssistant {
				out = append(out, thinkingEntry([]string{p.Data.Thinking})...)
			}
		case "tool_call":
			var input map[string]any
			_ = json.Unmarshal([]byte(p.Data.Input), &input)
			if e, ok := toolEntry(map[string]any{"name": p.Data.Name, "input": input}); ok && entryRole == RoleAssistant {
				out = append(out, e)
			}
		}
	}
	return out
}

// DeleteSession goes through the crush CLI: the database belongs to Crush.
// `crush session show --json` (metadata and every message) is the backup;
// without it nothing is deleted. Crush 0.96 finds the session from the
// working directory; its --cwd flag does not reach session commands.
func (c *Crush) DeleteSession(s Session, backupsDir string) error {
	if s.ID == "" || strings.ContainsAny(s.ID, `/\`) || strings.HasPrefix(s.ID, "-") {
		return fmt.Errorf("crush session id %q is not safe", s.ID)
	}
	bin, err := c.Look("crush")
	if err != nil {
		return fmt.Errorf("crush not found in PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if !dirExists(s.CWD) {
		return fmt.Errorf("crush session %s: project dir %s is gone", s.ID, s.CWD)
	}
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = s.CWD
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
	data, err := run("session", "show", s.ID, "--json")
	if err != nil {
		return fmt.Errorf("crush session show (backup before delete): %w", err)
	}
	if !json.Valid(bytes.TrimSpace(data)) {
		return fmt.Errorf("crush session show (backup before delete): output is not JSON")
	}
	backup := filepath.Join(backupsDir, fmt.Sprintf("crush-%s.%s.json", s.ID, time.Now().Format("20060102T150405.000000000")))
	if err := fsutil.WriteAtomic(backup, data, 0o600); err != nil {
		return fmt.Errorf("backing up crush session: %w", err)
	}
	if _, err := run("session", "delete", s.ID); err != nil {
		return fmt.Errorf("crush session delete: %w", err)
	}
	return nil
}
