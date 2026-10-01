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

// SessionUsage is the cost Crush summed for the session from its own prices.
// Its token counts are the last request's, not the session's, so none are
// given; a zero cost means an unpriced model, not a free one.
func (c *Crush) SessionUsage(s Session) (Usage, bool) {
	var rows []struct {
		Cost float64 `json:"cost"`
	}
	q := fmt.Sprintf(`SELECT cost FROM sessions WHERE id='%s'`, strings.ReplaceAll(s.ID, "'", "''"))
	if c.crushQuery(s.Path, q, &rows) != nil || len(rows) != 1 || rows[0].Cost <= 0 {
		return Usage{}, false
	}
	return Usage{Cost: rows[0].Cost, CostKnown: true}, true
}

func (c *Crush) ResumeCmd(s Session) ([]string, string, bool) {
	dir := s.CWD
	if !dirExists(dir) {
		dir = c.Home
	}
	return []string{"crush", "--session", s.ID}, dir, true // run in dir: Crush finds sessions from it
}

// Transcript reads the session's messages in order; each holds a JSON list of
// parts: text, reasoning, tool calls and results. A result only marks its call
// failed; a summary message (Crush's compaction) becomes an event.
func (c *Crush) Transcript(s Session) ([]Entry, error) {
	var rows []struct {
		Role    string `json:"role"`
		Parts   string `json:"parts"`
		Created int64  `json:"created_at"` // seconds
		Summary int    `json:"is_summary_message"`
	}
	id := strings.ReplaceAll(s.ID, "'", "''")
	q := `SELECT role, parts, created_at, %s FROM messages WHERE session_id='%s' ORDER BY created_at, rowid LIMIT %d`
	if err := c.crushQuery(s.Path, fmt.Sprintf(q, "is_summary_message", id, maxTranscriptEntries), &rows); err != nil {
		// databases from before is_summary_message existed
		if err2 := c.crushQuery(s.Path, fmt.Sprintf(q, "0 AS is_summary_message", id, maxTranscriptEntries), &rows); err2 != nil {
			return nil, err
		}
	}
	var out []Entry
	calls := map[string]int{} // tool_call id → index in out
	for _, r := range rows {
		at := time.Unix(r.Created, 0).UTC()
		if r.Summary != 0 {
			out = append(out, Entry{Role: RoleEvent, Text: eventCompacted, Time: at})
			continue
		}
		var refs []struct {
			Type string `json:"type"`
			Data struct {
				ID         string `json:"id"`
				ToolCallID string `json:"tool_call_id"`
				IsError    bool   `json:"is_error"`
			} `json:"data"`
		}
		_ = json.Unmarshal([]byte(r.Parts), &refs)
		var ids []string
		for _, p := range refs {
			switch {
			case p.Type == "tool_call":
				ids = append(ids, p.Data.ID)
			case p.Type == "tool_result" && p.Data.IsError:
				if i, ok := calls[p.Data.ToolCallID]; ok {
					out[i].Failed = true
				}
			}
		}
		for _, e := range crushParts(r.Role, r.Parts) {
			e.Time = at
			if e.Role == RoleTool && len(ids) > 0 {
				calls[ids[0]], ids = len(out), ids[1:]
			}
			out = append(out, e)
		}
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
// without it nothing is deleted. Both target the listed database with -D:
// crush 0.96 ignores --cwd in session commands, and run from any other dir
// it would open (and create) a .crush there.
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
	dataDir := filepath.Dir(s.Path)
	if !dirExists(dataDir) {
		return fmt.Errorf("crush session %s: data dir %s is gone", s.ID, dataDir)
	}
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, bin, append(args, "-D", dataDir)...)
		cmd.Dir = dataDir
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
