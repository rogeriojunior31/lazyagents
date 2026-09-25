package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeTranscript(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "-p", "s1.jsonl")
	writeFile(t, path,
		`{"type":"mode","mode":"normal"}
{"type":"user","isMeta":true,"message":{"role":"user","content":"<local-command>x</local-command>"}}
{"type":"user","message":{"role":"user","content":"Question one"}}
invalid line that is not JSON
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Answer **one**"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"ignored"}]}}
{"type":"user","message":{"role":"user","content":"Question two"}}
`)
	c := &Claude{Home: home, Look: noBin}
	got, err := c.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: "user", Text: "Question one"},
		{Role: "assistant", Text: "Answer **one**"},
		{Role: "user", Text: "Question two"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCodexTranscript(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "r.jsonl")
	writeFile(t, path,
		`{"type":"session_meta","payload":{"id":"x","cwd":"/tmp"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hi codex"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello!"}]}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<user_instructions>ignorar</user_instructions>"}]}}
`)
	c := &Codex{Home: home, Look: noBin}
	got, err := c.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "Hi codex" || got[1] != (Entry{Role: "assistant", Text: "Hello!"}) {
		t.Fatalf("entries: %+v", got)
	}
}

func TestGeminiTranscript(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gemini", "history", "p", "chats", "session-1.jsonl")
	writeFile(t, path,
		`{"sessionId":"abc","startTime":"2026-07-06T10:00:00Z"}
{"role":"user","parts":[{"text":"Hi gemini"}]}
{"role":"model","parts":[{"text":"Hello, human"}]}
`)
	g := &Gemini{Home: home, Look: noBin}
	got, err := g.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "Hi gemini" || got[1] != (Entry{Role: "assistant", Text: "Hello, human"}) {
		t.Fatalf("entries: %+v", got)
	}
}

func TestGeminiTranscriptSetWrapper(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gemini", "tmp", "p", "chats", "session-2.jsonl")
	writeFile(t, path,
		`{"sessionId":"abc"}
{"type":"user","content":[{"text":"plain"}]}
{"$set":{"messages":[{"type":"user","content":[{"text":"from the batch"}]},{"type":"gemini","content":[{"text":"batch answer"}]}]}}
{"$set":{"lastUpdated":"2026-05-12T18:09:58.424Z"}}
`)
	g := &Gemini{Home: home, Look: noBin}
	got, err := g.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: "user", Text: "plain"},
		{Role: "user", Text: "from the batch"},
		{Role: "assistant", Text: "batch answer"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTranscriptTruncatesHugeEntry(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "t.jsonl")
	huge := strings.Repeat("x", maxEntryRunes+100)
	writeFile(t, path, `{"role":"user","content":"`+huge+`"}`+"\n")
	got, err := jsonlTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0].Text, "[… message truncated]") {
		t.Fatalf("truncation failed: len=%d", len(got))
	}
}

func TestUnsupportedTranscript(t *testing.T) {
	if _, err := (&ClaudeDesktop{Home: t.TempDir(), Look: noBin}).Transcript(Session{}); err == nil {
		t.Error("claude-desktop should refuse the transcript")
	}
	if _, err := (&Hermes{Home: t.TempDir(), Look: noBin}).Transcript(Session{}); err == nil {
		t.Error("hermes should refuse the transcript")
	}
}

// Each line becomes blocks in the order the agent produced them: text reasoning,
// speech and tool calls (one line each), in the Claude, Codex, Gemini and
// OpenCode shapes. Empty or encrypted reasoning and tool results are left out.
func TestTranscriptBlocks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "s.jsonl")
	writeFile(t, path,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"need to see the files"},{"type":"text","text":"Listing."},{"type":"tool_use","name":"Bash","input":{"command":"ls   -la\n/tmp","description":"list"}},{"type":"text","text":"Done."}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"abc"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"huge output"}]}}
{"type":"response_item","payload":{"type":"reasoning","summary":[],"encrypted_content":"gAAA"}}
{"type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"Checking the tests"}]}}
{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{\"command\":[\"bash\",\"-lc\",\"go test ./...\"]}"}}
{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","input":"text(await tools.exec_command({cmd:\"pwd && rg --files -g 'go.mod'\",yield:1}))"}}
{"type":"gemini","content":"Finished.","thoughts":[{"subject":"Plan","description":"run the build"}]}
{"role":"assistant","parts":[{"type":"reasoning","text":"opencode thinking"},{"type":"text","text":"ok"}]}
`)
	got, err := jsonlTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: RoleThinking, Text: "need to see the files"},
		{Role: RoleAssistant, Text: "Listing."},
		{Role: RoleTool, Text: "Bash · ls -la /tmp"},
		{Role: RoleAssistant, Text: "Done."},
		{Role: RoleThinking, Text: "Checking the tests"},
		{Role: RoleTool, Text: "shell · bash -lc go test ./..."},
		{Role: RoleTool, Text: "exec · pwd && rg --files -g 'go.mod'"},
		{Role: RoleThinking, Text: "**Plan** run the build"},
		{Role: RoleAssistant, Text: "Finished."},
		{Role: RoleThinking, Text: "opencode thinking"},
		{Role: RoleAssistant, Text: "ok"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
