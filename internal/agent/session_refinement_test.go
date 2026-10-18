package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRecordedChildNavigation(t *testing.T) {
	for _, tc := range []struct{ id, version string }{{"opencode", "2.0.32"}, {"crush", "0.98.1"}} {
		t.Run(tc.id, func(t *testing.T) {
			dir := filepath.Join(fixturesDir, tc.id, tc.version, "session-links")
			home := t.TempDir()
			if err := copyFixtureHome(filepath.Join(dir, "home"), home); err != nil {
				if errors.Is(err, errNoSQLite) {
					t.Skip(err)
				}
				t.Fatal(err)
			}
			ad := ByID(All(home), tc.id)
			parents, err := ad.ListSessions()
			if err != nil || len(parents) != 1 {
				t.Fatalf("parents=%+v: %v", parents, err)
			}
			parent := parents[0]
			before, err := os.ReadFile(parent.Path)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := ad.Transcript(parent)
			if err != nil {
				t.Fatal(err)
			}
			var linked []Entry
			for _, e := range entries {
				if e.Sub != "" {
					linked = append(linked, e)
				}
			}
			if len(linked) != 1 || linked[0].Kind != ToolAgent || linked[0].SubID == "" || linked[0].Sub != parent.Path {
				t.Fatalf("missing exact native child link: %+v", entries)
			}
			child := parent
			child.ID, child.Path = linked[0].SubID, linked[0].Sub
			childEntries, err := ad.Transcript(child)
			if err != nil || len(childEntries) < 2 || childEntries[0].Role != RoleUser || childEntries[len(childEntries)-1].Text != "The command printed fixture." {
				t.Fatalf("child transcript=%+v: %v", childEntries, err)
			}
			for _, list := range [][]Entry{entries, childEntries} {
				for i := range list {
					list[i].Sub = filepath.ToSlash(list[i].Sub)
				}
			}
			got := struct{ Parent, Child []Entry }{entries, childEntries}
			raw, _ := json.MarshalIndent(got, "", "  ")
			raw = append(bytes.ReplaceAll(raw, []byte(filepath.ToSlash(home)), []byte("/home/user")), '\n')
			golden := filepath.Join(dir, "golden.json")
			if *update {
				if err := os.WriteFile(golden, raw, 0o644); err != nil {
					t.Fatal(err)
				}
			} else if want, err := os.ReadFile(golden); err != nil || !bytes.Equal(raw, want) {
				t.Fatalf("child golden mismatch: %v\n%s", err, raw)
			}
			after, err := os.ReadFile(parent.Path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("navigation modified the native database")
			}

			// Even a recorded child ID is not authority to open another parent's session.
			sqlite, _ := exec.LookPath("sqlite3")
			table, column := "session_v2", "parent_id"
			if tc.id == "crush" {
				table, column = "sessions", "parent_session_id"
			}
			q := "UPDATE " + table + " SET " + column + "='unrelated' WHERE id='" + child.ID + "'"
			if out, err := exec.Command(sqlite, parent.Path, q).CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			entries, err = ad.Transcript(parent)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if e.Sub != "" || e.SubID != "" {
					t.Fatalf("linked a child of another session: %+v", e)
				}
			}
		})
	}
}

func TestRecordedLifecycleProof(t *testing.T) {
	for _, tc := range []struct {
		id, version, sub string
		backups          int
	}{
		{"opencode", "2.0.32", "session-links", 2}, {"gemini-cli", "0.64.0", "session-lifecycle", 1},
	} {
		t.Run(tc.id, func(t *testing.T) {
			dir := filepath.Join(fixturesDir, tc.id, tc.version, tc.sub)
			var proof struct {
				Agent, Version, OS, Arch, Recorded, How, ID string
				Resume                                      []string
				BeforeUsers, AfterUsers, BackupCount        int
				BackupIDs                                   []string
				BackupPrivate, Deleted                      bool
			}
			raw, err := os.ReadFile(filepath.Join(dir, "lifecycle.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &proof); err != nil {
				t.Fatal(err)
			}
			if proof.Agent != tc.id || proof.Version != tc.version || proof.OS == "" || proof.Arch == "" || proof.Recorded == "" || proof.How == "" || proof.ID == "" || !proof.Deleted || !proof.BackupPrivate || proof.BackupCount != tc.backups || len(proof.BackupIDs) != tc.backups || proof.AfterUsers != proof.BeforeUsers+1 {
				t.Fatalf("incomplete lifecycle proof: %+v", proof)
			}
			home := t.TempDir()
			if err := copyFixtureHome(filepath.Join(dir, "home"), home); err != nil {
				if errors.Is(err, errNoSQLite) {
					t.Skip(err)
				}
				t.Fatal(err)
			}
			ad := ByID(All(home), tc.id)
			sessions, err := ad.ListSessions()
			if err != nil || len(sessions) != 1 || sessions[0].ID != proof.ID {
				t.Fatalf("snapshot does not match resumed session: %+v %v", sessions, err)
			}
			entries, err := ad.Transcript(sessions[0])
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, e := range entries {
				if e.Role == RoleUser {
					n++
				}
			}
			if n != proof.AfterUsers {
				t.Fatalf("recorded user turns=%d; proof=%d", n, proof.AfterUsers)
			}
		})
	}
}

func TestGeminiNativeClockIgnoresCopiedFileTime(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(fixturesDir, "gemini-cli", "0.64.0", "home")
	if err := copyFixtureHome(dir, home); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(home, ".gemini", "tmp", "*", "chats", "session-*"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	path := files[0]
	raw, _ := os.ReadFile(path)
	var started, updated time.Time
	for line := range bytes.SplitSeq(raw, []byte("\n")) {
		var m struct {
			Start   string `json:"startTime"`
			Updated string `json:"lastUpdated"`
			Set     struct {
				Updated string `json:"lastUpdated"`
			} `json:"$set"`
		}
		_ = json.Unmarshal(line, &m)
		if m.Start != "" {
			started, _ = time.Parse(time.RFC3339Nano, m.Start)
		}
		if m.Updated != "" {
			updated, _ = time.Parse(time.RFC3339Nano, m.Updated)
		}
		if m.Set.Updated != "" {
			updated, _ = time.Parse(time.RFC3339Nano, m.Set.Updated)
		}
	}
	if started.IsZero() || updated.IsZero() {
		t.Fatal("native recording has no clocks")
	}
	if err := os.Chtimes(path, time.Now(), time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(t.TempDir(), "index.gob")
	g := NewGemini(home)
	g.Index = NewIndex(indexPath)
	assert := func() {
		t.Helper()
		sessions, err := g.ListSessions()
		if err != nil || len(sessions) != 1 || !sessions[0].MTime.Equal(updated) {
			t.Fatalf("native time lost: %+v %v", sessions, err)
		}
		e, _ := g.Index.get(path)
		if !e.GeminiStarted.Equal(started) {
			t.Fatal("startTime lost")
		}
	}
	assert()
	g = NewGemini(home)
	g.Index = NewIndex(indexPath)
	assert()
}

func TestGeminiNewestNativeCopyWinsAndRefreshes(t *testing.T) {
	home := t.TempDir()
	history := filepath.Join(home, ".gemini", "history", "p", "chats", "session-one.jsonl")
	tmp := filepath.Join(home, ".gemini", "tmp", "p", "chats", "session-one.jsonl")
	writeLines(t, history, `{"sessionId":"one","startTime":"2026-10-18T09:00:00Z","lastUpdated":"2026-10-18T10:00:00Z"}`, `{"type":"user","content":"older"}`)
	writeLines(t, tmp, `{"sessionId":"one","startTime":"2026-10-18T09:00:00Z","lastUpdated":"2026-10-18T11:00:00Z"}`, `{"type":"user","content":"newer"}`)
	g := NewGemini(home)
	sessions, _ := g.ListSessions()
	if len(sessions) != 1 || sessions[0].Path != tmp {
		t.Fatalf("stale duplicate wins: %+v", sessions)
	}
	f, _ := os.OpenFile(history, os.O_APPEND|os.O_WRONLY, 0)
	if _, err := f.WriteString("{\"$set\":{\"lastUpdated\":\"2026-10-18T12:00:00Z\"}}\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	sessions, _ = g.ListSessions()
	if len(sessions) != 1 || sessions[0].Path != history || sessions[0].MTime.Hour() != 12 {
		t.Fatalf("append not reflected: %+v", sessions)
	}
}

func TestGeminiSnapshotsReplaceTextAndToolStatus(t *testing.T) {
	path := writeLines(t, filepath.Join(t.TempDir(), "s.jsonl"),
		`{"id":"u","type":"user","content":"hello"}`,
		`{"id":"a","type":"gemini","timestamp":"2026-10-18T10:00:00Z","content":"partial","toolCalls":[{"id":"c","name":"run_shell_command","args":{"command":"false"},"status":"executing"}]}`,
		`{"id":"a","type":"gemini","timestamp":"2026-10-18T10:00:00Z","content":"complete","toolCalls":[{"id":"c","name":"run_shell_command","args":{"command":"false"},"status":"error","timestamp":"2026-10-18T10:00:01Z"}]}`)
	entries, err := NewGemini(t.TempDir()).Transcript(Session{Path: path})
	if err != nil || len(entries) != 3 || entries[1].Text != "complete" || !entries[2].Failed || entries[2].Time.Second() != 1 {
		t.Fatalf("duplicate or stale snapshot: %+v %v", entries, err)
	}
}

func TestSessionFileBackupIsPrivateAndExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	content := []byte("private transcript\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	backups := t.TempDir()
	if err := deleteSessionFile(path, backups); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(backups)
	if len(files) != 1 {
		t.Fatal(files)
	}
	backup := filepath.Join(backups, files[0].Name())
	raw, _ := os.ReadFile(backup)
	info, _ := os.Stat(backup)
	if !bytes.Equal(raw, content) || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("backup is not exact/private")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("source remains")
	}
}

func TestGeminiResumePatchesAndRewind(t *testing.T) {
	path := writeLines(t, filepath.Join(t.TempDir(), "s.jsonl"),
		`{"id":"u","type":"user","content":"hello"}`,
		`{"id":"a","type":"gemini","content":"old"}`,
		`{"id":"b","type":"gemini","content":"remove me"}`,
		`{"$patch":{"updates":[{"id":"a","content":"updated"}],"removeIds":["b"],"orderIds":["a","u"]}}`,
		`{"id":"c","type":"user","content":"discarded"}`, `{"$rewindTo":"c"}`)
	entries, err := NewGemini(t.TempDir()).Transcript(Session{Path: path})
	if err != nil || len(entries) != 2 || entries[0].Text != "updated" || entries[1].Text != "hello" {
		t.Fatalf("patch/order/rewind=%+v %v", entries, err)
	}
}
