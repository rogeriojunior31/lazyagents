package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestOpenCodeV2OwnsMigratedDatabase(t *testing.T) {
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not installed")
	}
	home := t.TempDir()
	if err := copyFixtureHome(filepath.Join(fixturesDir, "opencode", "2.0.32", "home"), home); err != nil {
		t.Fatal(err)
	}
	o := &OpenCode{Home: home, Look: exec.LookPath}
	// Legacy rows can survive a V2 upgrade; they must not reappear as current sessions.
	cmd := exec.Command(sqlite, o.dbPath(), `CREATE TABLE session(id TEXT, directory TEXT, title TEXT, time_updated INTEGER, parent_id TEXT);
INSERT INTO session VALUES('legacy-only','/old','Old session',1,NULL);`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	before, err := os.ReadFile(o.dbPath())
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := o.ListSessions()
	if err != nil || len(sessions) != 1 || sessions[0].ID == "legacy-only" {
		t.Fatalf("ListSessions = %v, %v", sessions, err)
	}
	entries, err := o.Transcript(sessions[0])
	if err != nil || len(entries) != 3 || entries[1].Kind != ToolShell {
		t.Fatalf("Transcript = %v, %v", entries, err)
	}
	if entries, err := o.Transcript(Session{ID: "' OR 1=1 --"}); err != nil || len(entries) != 0 {
		t.Fatalf("unsafe session selection: %v, %v", entries, err)
	}
	after, err := os.ReadFile(o.dbPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("reading sessions modified the native database")
	}
	cmd = exec.Command(sqlite, o.dbPath(), `DELETE FROM session_v2`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if sessions, err := o.ListSessions(); err != nil || len(sessions) != 0 {
		t.Fatalf("resurrected legacy sessions: %v, %v", sessions, err)
	}
}

// A prompt that starts with "<" is the user's; only synthetic text is injected.
func TestOpenCodePart(t *testing.T) {
	user, assistant := `{"role":"user"}`, `{"role":"assistant"}`
	for _, tc := range []struct {
		msg, part string
		want      []Entry
	}{
		{user, `{"type":"text","text":"<div> is misaligned"}`, []Entry{{Role: RoleUser, Text: "<div> is misaligned"}}},
		{user, `{"type":"text","text":"injected","synthetic":true}`, nil},
		{assistant, `{"type":"text","text":"done"}`, []Entry{{Role: RoleAssistant, Text: "done"}}},
		{assistant, `{"type":"reasoning","text":"thinking it over"}`, []Entry{{Role: RoleThinking, Text: "thinking it over"}}},
		{assistant, `{"type":"tool","tool":"bash","state":{"input":{"command":"ls"}}}`, []Entry{{Role: RoleTool, Text: "bash · ls", Kind: ToolShell}}},
		{assistant, `{"type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"false"},"metadata":{"exit":1}}}`, []Entry{{Role: RoleTool, Text: "bash · false", Kind: ToolShell, Failed: true}}},
		{assistant, `{"type":"tool","tool":"read","state":{"status":"error","input":{"filePath":"/x"}}}`, []Entry{{Role: RoleTool, Text: "read · /x", Kind: ToolRead, Failed: true}}},
		{assistant, `{"type":"step-start"}`, nil},
	} {
		if got := openCodePart(tc.msg, tc.part); !slices.Equal(got, tc.want) {
			t.Errorf("%s = %v, want %v", tc.part, got, tc.want)
		}
	}
}

// An older database without the part table still gives the transcript kept in
// message.data.
func TestOpenCodeTranscriptWithoutPartTable(t *testing.T) {
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not installed")
	}
	home := t.TempDir()
	o := &OpenCode{Home: home, Look: exec.LookPath}
	db := o.dbPath()
	writeFile(t, filepath.Join(filepath.Dir(db), ".keep"), "")
	schema := `CREATE TABLE message (id text PRIMARY KEY, session_id text, time_created integer, data text);
INSERT INTO message VALUES ('m1','ses_1',1,'{"role":"user","content":"hello"}');
INSERT INTO message VALUES ('m2','ses_1',2,'{"role":"assistant","content":"hi"}');`
	cmd := exec.Command(sqlite, db)
	cmd.Stdin = strings.NewReader(schema)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	got, err := o.Transcript(Session{ID: "ses_1"})
	if want := []Entry{{Role: RoleUser, Text: "hello"}, {Role: RoleAssistant, Text: "hi"}}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("Transcript = %v, %v; want %v", got, err, want)
	}
}
