package agent

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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
		{assistant, `{"type":"tool","tool":"bash","state":{"input":{"command":"ls"}}}`, []Entry{{Role: RoleTool, Text: "bash · ls"}}},
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
