package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

const HistoryLimit = 200
const HistoryTargetLimit = 1000
const HistoryBackupLimit = 32
const historyBytes = 32 << 20

// History contains attempt metadata only. Details, state, errors and payloads
// from OperationResult deliberately never cross this persistence boundary.
type History struct {
	Version int            `json:"version"`
	Entries []HistoryEntry `json:"entries"`
}
type HistoryEntry struct {
	ID      string          `json:"id"`
	Time    time.Time       `json:"time"`
	Module  string          `json:"module"`
	Action  string          `json:"action"`
	Subject string          `json:"subject,omitempty"`
	Failed  bool            `json:"failed"`
	Targets []HistoryTarget `json:"targets"`
	Omitted int             `json:"omitted_targets,omitempty"`
	Backups []HistoryBackup `json:"backups"`
}
type HistoryTarget struct {
	ID      string        `json:"id"`
	Outcome TargetOutcome `json:"outcome"`
}
type HistoryBackup struct {
	ID     string `json:"id"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

func (p Paths) HistoryPath() string { return filepath.Join(p.DataDir, "operation-history.json") }

func historyAction(module, action string) bool {
	switch module {
	case "providers":
		return action == "apply" || action == "clear"
	case "hooks":
		return action == "enable" || action == "disable" || action == "command"
	case "skills":
		return slices.Contains([]string{"enable", "disable", "install", "retry-install", "update", "remove", "adopt", "restore", "profile", "batch-enable", "batch-disable", "batch-update", "batch-remove"}, action)
	case "recovery":
		return action == "restore"
	}
	return false
}

func historyID(s string) bool {
	return len(s) > 0 && len(s) <= 240 && !strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) || unicode.Is(unicode.Cf, r) })
}

var historyBackupName = regexp.MustCompile(`^([^/\\]+)\.(\d{8}T\d{6}\.\d{9})(?:\.tar\.gz)?$`)

func readHistory(path string) (History, []byte, error) {
	h := History{Version: 1, Entries: []HistoryEntry{}}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return h, nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > historyBytes {
		return h, nil, fmt.Errorf("operation history is unavailable or is not a bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return h, nil, fmt.Errorf("reading operation history failed")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, historyBytes+1))
	if err != nil || len(data) > historyBytes {
		return h, nil, fmt.Errorf("reading operation history failed")
	}
	if json.Unmarshal(data, &h) != nil || h.Version != 1 || len(h.Entries) > HistoryLimit {
		return h, nil, fmt.Errorf("invalid operation history; inspect or move the local history file before retrying")
	}
	seen := map[string]bool{}
	for _, e := range h.Entries {
		_, idErr := hex.DecodeString(e.ID)
		if idErr != nil || len(e.ID) != 32 || strings.ContainsAny(e.ID, " /\\") || seen[e.ID] || e.Time.IsZero() || !historyAction(e.Module, e.Action) || e.Subject != "" && !historyID(e.Subject) || len(e.Targets) > HistoryTargetLimit || len(e.Backups) > HistoryBackupLimit || e.Omitted < 0 {
			return h, nil, fmt.Errorf("invalid operation history metadata")
		}
		seen[e.ID] = true
		for _, t := range e.Targets {
			if !historyID(t.ID) || !slices.Contains([]TargetOutcome{Succeeded, Failed, Skipped}, t.Outcome) {
				return h, nil, fmt.Errorf("invalid operation history target")
			}
		}
		for _, b := range e.Backups {
			if !historyID(b.ID) || !historyBackupName.MatchString(b.ID) || !historyID(b.Target) || b.Kind != "config" && b.Kind != "skill" {
				return h, nil, fmt.Errorf("invalid operation history backup")
			}
		}
	}
	return h, data, nil
}

func ReadHistory(paths Paths) ([]HistoryEntry, error) {
	h, _, err := readHistory(paths.HistoryPath())
	if err != nil {
		return nil, err
	}
	return h.Entries, nil
}

// ponytail: one local metadata writer; use per-path queues only if multiple
// homes need independent throughput inside one process.
var historyMu sync.Mutex

// A short exclusive directory lock covers read/modify/rename, including clear.
// Never steal an old lock: a paused writer may still own it.
func changeHistory(paths Paths, reset bool, change func(*History)) error {
	historyMu.Lock()
	defer historyMu.Unlock()
	path := paths.HistoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating operation history folder failed")
	}
	lock := path + ".lock"
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := os.Mkdir(lock, 0o700); err == nil {
			break
		} else if !os.IsExist(err) {
			return fmt.Errorf("locking operation history failed")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("operation history is busy; if no lazyagents process is writing, remove operation-history.json.lock and retry")
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer os.Remove(lock)
	h, before, err := readHistory(path)
	if err != nil {
		if !reset {
			return err
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > historyBytes {
			return err
		}
		before, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading operation history failed")
		}
		h = History{Version: 1, Entries: []HistoryEntry{}}
	}
	change(&h)
	data, err := json.MarshalIndent(h, "", "  ")
	for len(data)+1 > historyBytes && len(h.Entries) > 1 {
		h.Entries = h.Entries[:len(h.Entries)-1]
		data, err = json.MarshalIndent(h, "", "  ")
	}
	if err != nil || len(data)+1 > historyBytes {
		return fmt.Errorf("operation history exceeds its storage limit")
	}
	if err := fsutil.WriteAtomicIfUnchanged(path, before, before != nil, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("saving operation history failed; another writer may have changed it")
	}
	return nil
}

func ClearHistory(paths Paths) error {
	return changeHistory(paths, true, func(h *History) { h.Entries = []HistoryEntry{} })
}

func appendHistory(paths Paths, e HistoryEntry) error {
	return changeHistory(paths, false, func(h *History) {
		h.Entries = append(h.Entries, e)
		slices.SortFunc(h.Entries, func(a, b HistoryEntry) int {
			if c := b.Time.Compare(a.Time); c != 0 {
				return c
			}
			return strings.Compare(b.ID, a.ID)
		})
		h.Entries = h.Entries[:min(HistoryLimit, len(h.Entries))]
	})
}

// TrackOperation is deferred around a service write, shared by CLI and TUI.
// Snapshot references are correlated by time, action and target, not authority
// to replay. Recovery must verify them against the current adapter and files.
func TrackOperation(paths Paths, module, action string, subject ...string) func(*OperationResult) {
	started := time.Now()
	return func(result *OperationResult) {
		result.HistoryWarning = ""
		if !historyAction(module, action) {
			result.HistoryWarning = "Operation finished; invalid history action"
			return
		}
		if len(result.Targets) == 0 {
			return
		} // preflight/no-op is not a write attempt
		e := HistoryEntry{Time: started.UTC(), Module: module, Action: action, Failed: result.AsError() != nil, Targets: []HistoryTarget{}, Backups: []HistoryBackup{}}
		// Random bytes avoid timestamp collisions between independent processes.
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			result.HistoryWarning = "Operation finished; history could not generate an ID"
			return
		}
		e.ID = fmt.Sprintf("%x", id)
		for _, t := range result.Targets[:min(len(result.Targets), HistoryTargetLimit)] {
			target := t.ID
			if !historyID(target) {
				target = "[invalid target ID]"
			}
			e.Targets = append(e.Targets, HistoryTarget{target, t.Outcome})
		}
		if len(subject) > 0 && historyID(subject[0]) {
			e.Subject = subject[0]
		}
		e.Omitted = max(0, len(result.Targets)-len(e.Targets))
		e.Backups = relatedBackups(paths, e, started, time.Now())
		if err := appendHistory(paths, e); err != nil {
			result.HistoryWarning = "Operation finished; " + err.Error() + ". Do not repeat successful targets to repair history."
		}
	}
}

func relatedBackups(paths Paths, e HistoryEntry, start, end time.Time) []HistoryBackup {
	out := []HistoryBackup{}
	if e.Module == "skills" && !slices.Contains([]string{"update", "remove", "adopt", "restore", "batch-update", "batch-remove"}, e.Action) {
		return out
	}
	// ponytail: scan the backup inventory per completed attempt; return exact
	// references from adapters if very large inventories make this expensive.
	files, _ := os.ReadDir(paths.BackupsDir())
	for _, file := range files {
		if !file.Type().IsRegular() || !historyID(file.Name()) || !historyBackupName.MatchString(file.Name()) {
			continue
		}
		path := filepath.Join(paths.BackupsDir(), file.Name())
		if strings.HasSuffix(file.Name(), ".tar.gz") {
			parts := historyBackupName.FindStringSubmatch(file.Name())
			name, stamp := parts[1], parts[2]
			at, err := time.ParseInLocation("20060102T150405.000000000", stamp, time.Local)
			if err != nil || at.Before(start) || at.After(end) {
				continue
			}
			for _, t := range e.Targets {
				if len(out) >= HistoryBackupLimit {
					break
				}
				if t.ID == name || strings.HasPrefix(t.ID, name+" / ") {
					out = append(out, HistoryBackup{file.Name(), t.ID, "skill"})
					break
				}
			}
			continue
		}
		metaPath := fsutil.BackupMetadataPath(path)
		info, err := os.Lstat(metaPath)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8192 {
			continue
		}
		data, err := os.ReadFile(metaPath)
		var r fsutil.BackupRecord
		if err != nil || json.Unmarshal(data, &r) != nil || r.Version != 1 || r.Time.Before(start) || r.Time.After(end) {
			continue
		}
		expected := map[string][]string{"providers": {"provider." + e.Action}, "hooks": {"hook.install", "hook.uninstall"}, "recovery": {"config.restore"}}
		if !slices.Contains(expected[e.Module], r.Operation) {
			continue
		}
		for _, t := range e.Targets {
			if t.ID != r.Agent || len(out) >= HistoryBackupLimit {
				continue
			}
			body, err := os.ReadFile(path)
			if err == nil && fmt.Sprintf("%x", sha256.Sum256(body)) == r.SHA256 {
				out = append(out, HistoryBackup{file.Name(), t.ID, "config"})
			}
			break
		}
	}
	return out
}

// TrackWrite adapts single-target services without persisting their error text.
func TrackWrite(paths Paths, module, action, target string) func(*error) {
	finish := TrackOperation(paths, module, action, target)
	return func(err *error) {
		r := OperationResult{Targets: []TargetResult{{ID: target, Outcome: Succeeded}}}
		if *err != nil {
			r.Targets[0].Outcome = Failed
		}
		finish(&r)
		if r.HistoryWarning != "" {
			*err = errors.Join(*err, errors.New(r.HistoryWarning))
		}
	}
}
