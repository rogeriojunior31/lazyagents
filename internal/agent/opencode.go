package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	a.ReadDirs = []string{
		a.ManagedDir,
		filepath.Join(o.Home, ".claude", "skills"),
		filepath.Join(o.Home, ".agents", "skills"),
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := strings.ReplaceAll(s.ID, "'", "''")
	q := fmt.Sprintf(`SELECT data FROM message WHERE session_id='%s'
		ORDER BY time_created ASC LIMIT %d`, id, maxTranscriptEntries)
	out, err := exec.CommandContext(ctx, sqlite, "-json", "-readonly", o.dbPath(), q).Output()
	if err != nil {
		return nil, fmt.Errorf("querying messages of %s: %w", s.ID, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	var rows []struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("reading sqlite3 output: %w", err)
	}
	var entries []Entry
	for _, r := range rows {
		if e, ok := entryFromLine([]byte(r.Data)); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// DeleteSession delegates to the opencode CLI: sessions live in SQLite and
// cannot be removed by moving a file.
func (o *OpenCode) DeleteSession(s Session, _ string) error {
	bin, err := o.Look("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "session", "delete", s.ID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("opencode session delete: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
