package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

func TestHistoryRetentionConcurrencyAndRedaction(t *testing.T) {
	p := PathsIn(t.TempDir())
	// More attempts than retention; concurrent instances use the disk lock.
	var wg sync.WaitGroup
	warnings := make(chan string, HistoryLimit+10)
	for worker := 0; worker < 8; worker++ {
		wg.Go(func() {
			for i := 0; i < 27; i++ {
				finish := TrackOperation(p, "providers", "apply", "public-profile")
				r := OperationResult{Err: errors.New("TOP_SECRET_TOKEN"), Targets: []TargetResult{{ID: "claude-code", Outcome: Succeeded, Detail: "RAW_CONFIG_BODY", State: "TRANSCRIPT_TEXT"}, {ID: "codex", Outcome: Failed, Detail: "IMPORTED_COMMAND"}}}
				finish(&r)
				if r.HistoryWarning != "" {
					warnings <- r.HistoryWarning
				}
			}
		})
	}
	wg.Wait()
	close(warnings)
	for warning := range warnings {
		t.Error(warning)
	}
	entries, err := ReadHistory(p)
	if err != nil || len(entries) != HistoryLimit {
		t.Fatalf("entries=%d err=%v", len(entries), err)
	}
	ids := map[string]bool{}
	for i, e := range entries {
		if ids[e.ID] || !e.Failed || e.Subject != "public-profile" || len(e.Targets) != 2 || e.Targets[1].Outcome != Failed {
			t.Fatalf("invalid metadata: %+v", e)
		}
		ids[e.ID] = true
		if i > 0 && entries[i-1].Time.Before(e.Time) {
			t.Fatal("history is not newest first")
		}
	}
	data, _ := os.ReadFile(p.HistoryPath())
	for _, s := range []string{"TOP_SECRET_TOKEN", "RAW_CONFIG_BODY", "TRANSCRIPT_TEXT", "IMPORTED_COMMAND"} {
		if strings.Contains(string(data), s) {
			t.Fatalf("history leaked %s", s)
		}
	}
	info, _ := os.Stat(p.HistoryPath())
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("history is not owner only")
	}
}

func TestHistoryDoesNotLoseConcurrentRecordsBelowRetention(t *testing.T) {
	p := PathsIn(t.TempDir())
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Go(func() {
			r := OperationResult{Targets: []TargetResult{{ID: fmt.Sprintf("agent-%d", i), Outcome: Succeeded}}}
			TrackOperation(p, "hooks", "enable")(&r)
			if r.HistoryWarning != "" {
				t.Error(r.HistoryWarning)
			}
		})
	}
	wg.Wait()
	entries, err := ReadHistory(p)
	if err != nil || len(entries) != 24 {
		t.Fatalf("lost records: %d, %v", len(entries), err)
	}
}

func TestHistoryBackupReferencesAndIndependentClear(t *testing.T) {
	p := PathsIn(t.TempDir())
	config := filepath.Join(p.Home, "settings.json")
	if err := fsutil.WriteAtomic(config, []byte(`{"token":"PRIVATE"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	finish := TrackOperation(p, "providers", "apply")
	backup, err := fsutil.Backup(config, p.BackupsDir(), fsutil.BackupOrigin{Agent: "claude-code", Operation: "provider.apply"})
	if err != nil {
		t.Fatal(err)
	}
	// Same timestamp interval, but an unrelated action cannot become a reference.
	if _, err := fsutil.Backup(config, p.BackupsDir(), fsutil.BackupOrigin{Agent: "codex", Operation: "hook.install"}); err != nil {
		t.Fatal(err)
	}
	r := OperationResult{Targets: []TargetResult{{ID: "claude-code", Outcome: Succeeded}, {ID: "codex", Outcome: Failed}}}
	finish(&r)
	entries, err := ReadHistory(p)
	if err != nil || len(entries) != 1 || len(entries[0].Backups) != 1 || entries[0].Backups[0].ID != filepath.Base(backup) {
		t.Fatalf("references=%+v err=%v", entries, err)
	}
	if err := ClearHistory(p); err != nil {
		t.Fatal(err)
	}
	entries, err = ReadHistory(p)
	if err != nil || len(entries) != 0 {
		t.Fatal("history not cleared")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatal("clear removed backup")
	}
}

func TestHistoryFailureDoesNotTurnSuccessIntoRetry(t *testing.T) {
	p := PathsIn(t.TempDir())
	if err := fsutil.WriteAtomic(p.HistoryPath(), []byte("{PRIVATE broken json"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := OperationResult{Targets: []TargetResult{{ID: "claude-code", Outcome: Succeeded}}}
	TrackOperation(p, "providers", "clear")(&r)
	if r.HistoryWarning == "" || strings.Contains(r.HistoryWarning, "PRIVATE") || r.AsError() != nil || r.CompletionError() == nil || len(r.FailedIDs()) != 0 {
		t.Fatalf("invalid warning/result: %+v", r)
	}
	if entries, err := ReadHistory(p); err == nil || entries != nil {
		t.Fatal("corrupt history exposed partial records")
	}
	if err := ClearHistory(p); err != nil {
		t.Fatal("cannot explicitly clear corrupt history", err)
	}
	TrackOperation(p, "providers", "clear")(&r)
	entries, err := ReadHistory(p)
	if err != nil || len(entries) != 1 {
		t.Fatal("recording did not resume")
	}
}

func TestHistoryBoundsAndUntrustedMetadata(t *testing.T) {
	p := PathsIn(t.TempDir())
	r := OperationResult{Targets: make([]TargetResult, HistoryTargetLimit+2)}
	for i := range r.Targets {
		r.Targets[i] = TargetResult{ID: "skill / claude-code", Outcome: Skipped}
	}
	r.Targets[0].ID = "\x1b[31m"
	TrackOperation(p, "skills", "install")(&r)
	entries, err := ReadHistory(p)
	if err != nil || len(entries) != 1 || entries[0].Omitted != 2 || entries[0].Targets[0].ID != "[invalid target ID]" {
		t.Fatalf("bounds: %v %+v", err, entries)
	}
	// Parsing rejects controls even in top-level attempt identities.
	data, _ := os.ReadFile(p.HistoryPath())
	data = []byte(strings.Replace(string(data), entries[0].ID, strings.Repeat("x", 32), 1))
	if err := fsutil.WriteAtomic(p.HistoryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHistory(p); err == nil {
		t.Fatal("invalid identity accepted")
	}
	if err := ClearHistory(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p.HistoryPath()+".lock", 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	r = OperationResult{Targets: []TargetResult{{ID: "claude-code", Outcome: Succeeded}}}
	TrackOperation(p, "providers", "apply")(&r)
	if !strings.Contains(r.HistoryWarning, "busy") || time.Since(started) > 4*time.Second {
		t.Fatal("lock not bounded/reported")
	}
	if err := os.Remove(p.HistoryPath() + ".lock"); err != nil {
		t.Fatal(err)
	}
}
