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

// OpenCode adapta o opencode. Skills em ~/.config/opencode/skills (e ele
// também lê ~/.claude/skills e ~/.agents/skills); sessões num SQLite em
// ~/.local/share/opencode/opencode.db, lido via binário sqlite3 (sem cgo).
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
		a.Detail = "config em " + o.configDir() + " (binário fora do PATH)"
	} else {
		a.Detail = "não instalado"
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
		return nil, fmt.Errorf("sessões do opencode ficam num SQLite; instale o sqlite3 para listá-las")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const q = `SELECT id, directory, title, time_updated FROM session
		WHERE parent_id IS NULL ORDER BY time_updated DESC LIMIT 500`
	out, err := exec.CommandContext(ctx, sqlite, "-json", "-readonly", db, q).Output()
	if err != nil {
		return nil, fmt.Errorf("consultando %s: %w", db, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	var rows []opencodeRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("lendo resultado do sqlite3: %w", err)
	}
	sessions := make([]Session, 0, len(rows))
	for _, r := range rows {
		title := cleanTitle(r.Title, 80)
		if title == "" {
			title = "(sem título)"
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
	// retorna o CWD original sem fallback; quem chama decide o que fazer com dir inválido
	return []string{"opencode", "--session", s.ID}, s.CWD, true
}

// ID implementa Adapter sem I/O.
func (o *OpenCode) ID() string { return "opencode" }

// Transcript consulta as mensagens da sessão no SQLite via sqlite3. O campo
// data é um JSON por mensagem, parseado de forma tolerante.
func (o *OpenCode) Transcript(s Session) ([]Entry, error) {
	sqlite, err := o.Look("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("transcript do opencode requer o binário sqlite3")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := strings.ReplaceAll(s.ID, "'", "''")
	q := fmt.Sprintf(`SELECT data FROM message WHERE session_id='%s'
		ORDER BY time_created ASC LIMIT %d`, id, maxTranscriptEntries)
	out, err := exec.CommandContext(ctx, sqlite, "-json", "-readonly", o.dbPath(), q).Output()
	if err != nil {
		return nil, fmt.Errorf("consultando mensagens de %s: %w", s.ID, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	var rows []struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("lendo resultado do sqlite3: %w", err)
	}
	var entries []Entry
	for _, r := range rows {
		if e, ok := entryFromLine([]byte(r.Data)); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// DeleteSession delega ao CLI do opencode: as sessões ficam num SQLite e
// não podem ser removidas movendo um arquivo individual.
func (o *OpenCode) DeleteSession(s Session, _ string) error {
	bin, err := o.Look("opencode")
	if err != nil {
		return fmt.Errorf("opencode não encontrado no PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "session", "delete", s.ID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("opencode session delete: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
