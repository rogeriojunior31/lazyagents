// Package recovery previews and restores agent configuration snapshots.
package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

type Service struct {
	adapters []agent.Adapter
	paths    core.Paths
	dir      string
}

func New(adapters []agent.Adapter, backupsDir string) *Service {
	return &Service{adapters: adapters, dir: backupsDir, paths: core.Paths{DataDir: filepath.Dir(backupsDir)}}
}

type Entry struct {
	ID          string    `json:"id"`
	Agent       string    `json:"agent"`
	Operation   string    `json:"operation"`
	Destination string    `json:"destination,omitempty"`
	Time        time.Time `json:"time"`
	Restorable  bool      `json:"restorable"`
	Reason      string    `json:"reason,omitempty"`
}

// Preview carries no configuration values; Review binds the exact source,
// provenance, destination contents, permissions and resolved parent directory.
type Preview struct {
	Entry
	Changes       []string `json:"changes"`
	CurrentExists bool     `json:"current_exists"`
	CurrentMode   string   `json:"current_mode"`
	RestoredMode  string   `json:"restored_mode"`
	Review        string   `json:"review"`
}

var snapshotName = regexp.MustCompile(`^(.+)\.(\d{8}T\d{6}(?:\.\d{9})?)$`)

func validOperation(op string) bool {
	switch op {
	case "provider.apply", "provider.clear", "hook.install", "hook.uninstall", "config.restore":
		return true
	}
	return false
}

func (s *Service) target(record fsutil.BackupRecord) (agent.RecoveryFile, bool) {
	for _, ad := range s.adapters {
		host, ok := ad.(agent.ConfigRecoveryHost)
		if !ok || ad.ID() != record.Agent {
			continue
		}
		for _, file := range host.RecoveryFiles() {
			if file.Path == record.Destination && filepath.IsAbs(file.Path) {
				return file, true
			}
		}
	}
	return agent.RecoveryFile{}, false
}

func (s *Service) readRecord(id string) (fsutil.BackupRecord, []byte, error) {
	if filepath.Base(id) != id || !snapshotName.MatchString(id) {
		return fsutil.BackupRecord{}, nil, fmt.Errorf("invalid config backup ID")
	}
	data, _, err := regularFile(fsutil.BackupMetadataPath(filepath.Join(s.dir, id)), false)
	if err != nil {
		return fsutil.BackupRecord{}, nil, fmt.Errorf("backup origin unavailable; legacy or damaged metadata")
	}
	var r fsutil.BackupRecord
	if err := json.Unmarshal(data, &r); err != nil || r.Version != 1 || r.Mode > 0o777 || len(r.SHA256) != 64 || !validOperation(r.Operation) {
		return r, nil, fmt.Errorf("invalid backup provenance")
	}
	if _, ok := s.target(r); !ok {
		return r, nil, fmt.Errorf("backup destination is not approved by the current agent adapter")
	}
	if parts := snapshotName.FindStringSubmatch(id); r.Time.IsZero() || parts[1] != filepath.Base(r.Destination) {
		return r, nil, fmt.Errorf("backup provenance does not match its file identity")
	}
	return r, data, nil
}

func (s *Service) List() ([]Entry, error) {
	files, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config backups: %w", err)
	}
	basenames := map[string]bool{}
	for _, ad := range s.adapters {
		if host, ok := ad.(agent.ConfigRecoveryHost); ok {
			for _, file := range host.RecoveryFiles() {
				basenames[filepath.Base(file.Path)] = true
			}
		}
	}
	out := []Entry{}
	for _, file := range files {
		parts := snapshotName.FindStringSubmatch(file.Name())
		if file.IsDir() || len(parts) != 3 || !basenames[parts[1]] {
			continue // skill archives and session backups are separate
		}
		at, _ := time.ParseInLocation("20060102T150405.000000000", parts[2], time.Local)
		if at.IsZero() {
			at, _ = time.ParseInLocation("20060102T150405", parts[2], time.Local)
		}
		entry := Entry{ID: file.Name(), Agent: "unknown", Operation: "unknown", Time: at}
		r, _, err := s.readRecord(entry.ID)
		if err != nil {
			entry.Reason = err.Error()
		} else {
			entry.Agent, entry.Operation, entry.Destination, entry.Time = r.Agent, r.Operation, r.Destination, r.Time
			entry.Restorable = file.Type().IsRegular()
			if !entry.Restorable {
				entry.Reason = "backup is not a regular file"
			}
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

func regularFile(path string, allowMissing bool) ([]byte, os.FileMode, error) {
	info, err := os.Lstat(path)
	if allowMissing && os.IsNotExist(err) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("expected a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	return data, info.Mode().Perm(), err
}

// parentIdentity handles deleted config directories while binding any existing
// symlinked ancestors. Replacing an ancestor invalidates the review token.
func parentIdentity(path string) (string, error) {
	dir, suffix := filepath.Dir(path), ""
	for {
		real, err := filepath.EvalSymlinks(dir)
		if err == nil {
			return filepath.Join(real, suffix), nil
		}
		if !os.IsNotExist(err) || filepath.Dir(dir) == dir {
			return "", err
		}
		suffix = filepath.Join(filepath.Base(dir), suffix)
		dir = filepath.Dir(dir)
	}
}

func fingerprint(parts ...[]byte) string {
	h := sha256.New()
	for _, part := range parts {
		fmt.Fprintf(h, "%d:", len(part))
		h.Write(part)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// prepare keeps raw bytes inside the service, never in an exported plan or UI.
func (s *Service) prepare(id string) (Preview, []byte, []byte, bool, os.FileMode, error) {
	r, metadata, err := s.readRecord(id)
	if err != nil {
		return Preview{}, nil, nil, false, 0, err
	}
	file, _ := s.target(r)
	backupPath := filepath.Join(s.dir, id)
	archived, _, err := regularFile(backupPath, false)
	if err != nil {
		return Preview{}, nil, nil, false, 0, fmt.Errorf("reading backup: %w", err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(archived)) != r.SHA256 {
		return Preview{}, nil, nil, false, 0, fmt.Errorf("backup integrity check failed; nothing restored")
	}
	current, mode, err := regularFile(file.Path, true)
	if err != nil {
		return Preview{}, nil, nil, false, 0, fmt.Errorf("reading destination: %w", err)
	}
	_, statErr := os.Lstat(file.Path)
	existed := statErr == nil
	parent, err := parentIdentity(file.Path)
	if err != nil {
		return Preview{}, nil, nil, false, 0, fmt.Errorf("resolving destination: %w", err)
	}
	changes, err := file.Changes(current, archived)
	if err != nil {
		return Preview{}, nil, nil, false, 0, err
	}
	perm := os.FileMode(r.Mode)
	if existed {
		perm &= mode // restoring credentials must never broaden access
	}
	entry := Entry{ID: id, Agent: r.Agent, Operation: r.Operation, Destination: file.Path, Time: r.Time, Restorable: true}
	preview := Preview{Entry: entry, Changes: changes, CurrentExists: existed, CurrentMode: fmt.Sprintf("%04o", mode), RestoredMode: fmt.Sprintf("%04o", perm)}
	state := fmt.Sprintf("%t:%o:%s", existed, mode, parent)
	preview.Review = fingerprint([]byte(id), metadata, archived, current, []byte(state))
	return preview, current, archived, existed, perm, nil
}

func (s *Service) Preview(id string) (Preview, error) {
	p, _, _, _, _, err := s.prepare(id)
	return p, err
}

func (s *Service) Restore(id, review string) (restoreErr error) {
	p, current, archived, existed, mode, err := s.prepare(id)
	if err != nil {
		return err
	}
	if review == "" || p.Review != review {
		return fmt.Errorf("restore review is missing or stale; preview this backup again")
	}
	if bytes.Equal(current, archived) && p.CurrentMode == p.RestoredMode && existed {
		return nil
	}
	defer core.TrackWrite(s.paths, "recovery", "restore", p.Agent)(&restoreErr)
	if existed {
		if _, err := fsutil.Backup(p.Destination, s.dir, fsutil.BackupOrigin{Agent: p.Agent, Operation: "config.restore"}); err != nil {
			return err
		}
	}
	// Bind the related files and permissions at the final rename check too.
	guard := func() error {
		now, err := s.Preview(id)
		if err != nil || now.Review != review {
			return fmt.Errorf("restore source or destination changed; preview again: %w", fsutil.ErrChanged)
		}
		return nil
	}
	if err := fsutil.WriteAtomicIfUnchanged(p.Destination, current, existed, archived, mode, guard); err != nil {
		return fmt.Errorf("restoring config: %w", err)
	}
	_ = fsutil.RotateBackups(s.dir, filepath.Base(p.Destination)+".", 20)
	return nil
}

func Filter(entries []Entry, query string) []Entry {
	terms := strings.Fields(strings.ToLower(query))
	out := []Entry{}
	for _, entry := range entries {
		text := strings.ToLower(entry.ID + " " + entry.Agent + " " + entry.Operation + " " + entry.Destination + " " + entry.Time.Local().Format("2006-01-02 15:04:05"))
		matches := true
		for _, term := range terms {
			matches = matches && strings.Contains(text, term)
		}
		if matches {
			out = append(out, entry)
		}
	}
	return out
}
