package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// crushProject writes projects.json and a crush.db with the given SQL, for a
// project inside home; it needs the sqlite3 binary, like the adapter.
func crushProjectDB(t *testing.T, home, sql string) (*Crush, string) {
	t.Helper()
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not installed")
	}
	proj := filepath.Join(home, "proj")
	db := filepath.Join(proj, ".crush", "crush.db")
	mkdirs(t, filepath.Dir(db))
	cmd := exec.Command(sqlite, db)
	cmd.Stdin = strings.NewReader(`CREATE TABLE sessions (id TEXT PRIMARY KEY, parent_session_id TEXT, title TEXT NOT NULL,
  message_count INTEGER, prompt_tokens INTEGER, completion_tokens INTEGER, cost REAL, updated_at INTEGER NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE messages (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, role TEXT NOT NULL, parts TEXT NOT NULL, created_at INTEGER NOT NULL);
` + sql)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	c := &Crush{Home: home, Look: exec.LookPath}
	idx := `{"projects":[{"path":` + quoteJSON(filepath.ToSlash(proj)) + `,"data_dir":` + quoteJSON(filepath.ToSlash(filepath.Dir(db))) + `}]}`
	writeFile(t, filepath.Join(c.dataDir(), "projects.json"), idx)
	return c, proj
}

func quoteJSON(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

// A sub-agent's session belongs to the one that started it: only top-level
// sessions are listed; reasoning parts become thinking entries.
func TestCrushHidesSubAgentSessions(t *testing.T) {
	c, proj := crushProjectDB(t, t.TempDir(), `
INSERT INTO sessions VALUES ('main','','Main task',2,0,0,0,200,100);
INSERT INTO sessions VALUES ('sub','main','Sub-agent task',1,0,0,0,300,150);
INSERT INTO messages VALUES ('m1','main','user','[{"type":"text","data":{"text":"do it"}}]',101);
INSERT INTO messages VALUES ('m2','main','assistant','[{"type":"reasoning","data":{"thinking":"plan first"}},{"type":"text","data":{"text":"done"}}]',102);
`)
	list, err := c.ListSessions()
	if err != nil || len(list) != 1 || list[0].ID != "main" || list[0].CWD != filepath.ToSlash(proj) {
		t.Fatalf("ListSessions = %+v, %v", list, err)
	}
	got, err := c.Transcript(list[0])
	want := []Entry{{Role: RoleUser, Text: "do it"}, {Role: RoleThinking, Text: "plan first"}, {Role: RoleAssistant, Text: "done"}}
	if err != nil || len(got) != len(want) {
		t.Fatalf("Transcript = %v, %v", got, err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// Delete backs the session up with `crush session show --json` first, and
// without a valid backup deletes nothing.
func TestCrushDeleteBacksUpFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in")
	}
	for _, tc := range []struct {
		show    string
		deletes bool
	}{
		{`{"id":"s1","messages":[]}`, true},
		{`not json`, false},
	} {
		dir := t.TempDir()
		marker := filepath.Join(dir, "deleted")
		script := "#!/bin/sh\n" +
			"if [ \"$2\" = show ]; then printf '%s' '" + tc.show + "'; fi\n" +
			"if [ \"$2\" = delete ]; then echo \"$@\" > '" + marker + "'; fi\n"
		bin := filepath.Join(dir, "crush")
		if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		c := &Crush{Home: t.TempDir(), Look: func(string) (string, error) { return bin, nil }}
		backups := t.TempDir()
		data := t.TempDir()
		err := c.DeleteSession(Session{ID: "s1", CWD: "/elsewhere", Path: filepath.Join(data, "crush.db")}, backups)
		args, _ := os.ReadFile(marker)
		entries, _ := os.ReadDir(backups)
		switch {
		case tc.deletes && (err != nil || len(entries) != 1 || strings.TrimSpace(string(args)) != "session delete s1 -D "+data):
			t.Errorf("valid backup: err %v, backups %d, delete args %q", err, len(entries), args)
		case !tc.deletes && (err == nil || len(args) != 0):
			t.Errorf("invalid backup: err %v, delete ran: %q", err, args)
		}
	}
	if err := (&Crush{Look: noBin}).DeleteSession(Session{ID: "--all"}, t.TempDir()); err == nil {
		t.Error("an id that reads as a flag must be refused")
	}
}
