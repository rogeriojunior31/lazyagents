package agent

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
		{Role: RoleTool, Text: "Bash · ls -la /tmp", Kind: ToolShell},
		{Role: RoleAssistant, Text: "Done."},
		{Role: RoleThinking, Text: "Checking the tests"},
		{Role: RoleTool, Text: "shell · bash -lc go test ./...", Kind: ToolShell},
		{Role: RoleTool, Text: "exec · pwd && rg --files -g 'go.mod'", Kind: ToolShell},
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

// Times, failed calls and session events, as Claude Code 2.1 writes them.
func TestClaudeTranscriptEvents(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "-p", "s1.jsonl")
	writeFile(t, path,
		`{"type":"user","timestamp":"2026-10-01T09:04:00.000Z","message":{"role":"user","content":"run it"}}
{"type":"assistant","timestamp":"2026-10-01T09:04:05.000Z","message":{"role":"assistant","model":"claude-opus-5-5","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"go test"}}]}}
{"type":"assistant","timestamp":"2026-10-01T09:04:06.000Z","message":{"role":"assistant","model":"claude-opus-5-5","content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"go vet"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"Exit code 1"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"ok"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}
{"type":"user","message":{"role":"user","content":"<command-name>/plan</command-name>\n<command-message>plan</command-message>\n<command-args>split the work</command-args>"}}
{"type":"assistant","message":{"role":"assistant","model":"<synthetic>","content":[{"type":"text","text":"API error"}]}}
{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"Plan ready."}]}}
{"type":"system","subtype":"compact_boundary","content":"Conversation compacted"}
{"type":"user","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued…"}}
`)
	c := &Claude{Home: home, Look: noBin}
	got, err := c.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 9, 4, 0, 0, time.UTC)
	want := []Entry{
		{Role: RoleUser, Text: "run it", Time: at},
		{Role: RoleTool, Text: "Bash · go test", Time: at.Add(5 * time.Second), Failed: true, Kind: ToolShell},
		{Role: RoleTool, Text: "Bash · go vet", Time: at.Add(6 * time.Second), Kind: ToolShell},
		{Role: RoleEvent, Text: "interrupted by the user"},
		{Role: RoleUser, Text: "/plan split the work"},
		{Role: RoleAssistant, Text: "API error"},
		{Role: RoleEvent, Text: "model changed to claude-sonnet-5-5"},
		{Role: RoleAssistant, Text: "Plan ready."},
		{Role: RoleEvent, Text: "context compacted"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("entries:\n got %+v\nwant %+v", got, want)
	}
}

func TestCodexTranscriptEvents(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "r.jsonl")
	writeFile(t, path,
		`{"type":"turn_context","payload":{"model":"gpt-6-astra"}}
{"timestamp":"2026-09-21T02:51:00.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hi"}]}}
{"type":"event_msg","payload":{"type":"turn_aborted","reason":"interrupted"}}
{"type":"turn_context","payload":{"model":"gpt-6-astra"}}
{"type":"compacted","payload":{"message":""}}
{"type":"turn_context","payload":{"model":"gpt-6-mini"}}
`)
	c := &Codex{Home: home, Look: noBin}
	got, err := c.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: RoleUser, Text: "Hi", Time: time.Date(2026, 9, 21, 2, 51, 0, 0, time.UTC)},
		{Role: RoleEvent, Text: "interrupted by the user"},
		{Role: RoleEvent, Text: "context compacted"},
		{Role: RoleEvent, Text: "model changed to gpt-6-mini"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("entries:\n got %+v\nwant %+v", got, want)
	}
}

// withoutTimes compares transcripts by content where the time is not the point.
func withoutTimes(es []Entry) []Entry {
	out := slices.Clone(es)
	for i := range out {
		out[i].Time = time.Time{}
	}
	return out
}

// Each call gets its kind and the detail the reader shows for it.
func TestToolKinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	patch := `*** Begin Patch\n*** Update File: /p/a.go\n@@\n-old\n+new\n+more\n*** Add File: /p/b.go\n+x\n*** End Patch`
	writeFile(t, path, `{"type":"assistant","message":{"role":"assistant","content":[`+
		`{"type":"tool_use","id":"1","name":"Edit","input":{"file_path":"/p/a.go","old_string":"a\nb\nc","new_string":"a\nB\nB2\nc"}},`+
		`{"type":"tool_use","id":"2","name":"Write","input":{"file_path":"/p/n.go","content":"1\n2\n3\n"}},`+
		`{"type":"tool_use","id":"3","name":"Read","input":{"file_path":"/p/a.go"}},`+
		`{"type":"tool_use","id":"4","name":"Agent","input":{"description":"Survey adapters","prompt":"…","subagent_type":"Explore"}},`+
		`{"type":"tool_use","id":"5","name":"ExitPlanMode","input":{"plan":"# Plan\n1. do it"}},`+
		`{"type":"tool_use","id":"6","name":"TodoWrite","input":{"todos":[{"content":"write","status":"completed"},{"content":"test","status":"in_progress"},{"content":"ship","status":"pending"}]}},`+
		`{"type":"tool_use","id":"7","name":"AskUserQuestion","input":{"questions":[]}}]}}
{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","input":"text(await tools.apply_patch(\"`+strings.ReplaceAll(patch, `\n`, `\\n`)+`\"))"}}
{"type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","input":"`+patch+`"}}
`)
	got, err := jsonlTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: RoleTool, Text: "Edit · /p/a.go", Kind: ToolEdit, Added: 2, Removed: 1},
		{Role: RoleTool, Text: "Write · /p/n.go", Kind: ToolEdit, Added: 3},
		{Role: RoleTool, Text: "Read · /p/a.go", Kind: ToolRead},
		{Role: RoleTool, Text: "Agent · Survey adapters", Kind: ToolAgent},
		{Role: RoleTool, Text: "ExitPlanMode", Kind: ToolPlan, Body: "# Plan\n1. do it"},
		{Role: RoleTool, Text: "TodoWrite", Kind: ToolTodo, Body: "☑ write\n◐ test\n☐ ship"},
		{Role: RoleTool, Text: "AskUserQuestion"},
		{Role: RoleTool, Text: "exec · /p/a.go, /p/b.go", Kind: ToolEdit, Added: 3, Removed: 1},
		{Role: RoleTool, Text: "apply_patch · /p/a.go, /p/b.go", Kind: ToolEdit, Added: 3, Removed: 1},
	}
	if !slices.Equal(got, want) {
		t.Errorf("entries:\n got %+v\nwant %+v", got, want)
	}
}
