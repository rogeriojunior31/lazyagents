package sessions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestExportMarkdown(t *testing.T) {
	dir := t.TempDir()
	s := agent.Session{
		AgentID: "claude-code", AgentName: "Claude Code",
		ID: "sess-1", CWD: "/tmp/proj",
		Title: "my session", MTime: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC),
	}
	entries := []agent.Entry{
		{Role: "user", Text: "how do I do X?"},
		{Role: "assistant", Text: "do Y."},
		{Role: agent.RoleTool, Text: "Bash · go test ./..."},
		{Role: agent.RoleTool, Text: "Bash · go vet", Failed: true},
		{Role: agent.RoleTool, Text: "Edit · a.go", Kind: agent.ToolEdit, Added: 2, Removed: 1},
		{Role: agent.RoleTool, Text: "TodoWrite", Kind: agent.ToolTodo, Body: "☑ write\n☐ test"},
		{Role: agent.RoleThinking, Text: "line 1\nline 2"},
		{Role: agent.RoleEvent, Text: "context compacted"},
	}

	path, err := ExportMarkdown(s, entries, dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("path outside the expected dir: %s", path)
	}
	if !strings.HasPrefix(filepath.Base(path), "claude-code-sess-1-") || !strings.HasSuffix(path, ".md") {
		t.Errorf("unexpected file name: %s", filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"my session", "Claude Code", "/tmp/proj", "▶ you", "how do I do X?", "◀ Claude Code", "do Y.", "- ❯ `Bash · go test ./...`\n", "- ❯ `Bash · go vet` ✗", "- ❯ `Edit · a.go` (+2 −1)", "    ☑ write\n    ☐ test", "> 💭 line 1\n> line 2", "_— context compacted —_"} {
		if !strings.Contains(content, want) {
			t.Errorf("export lacks %q:\n%s", want, content)
		}
	}

	// empty transcript: error, no file
	before, _ := os.ReadDir(dir)
	if _, err := ExportMarkdown(s, nil, dir); err == nil {
		t.Fatal("empty transcript should be an error")
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before) {
		t.Fatal("empty transcript should not write a file")
	}
}

func TestServiceExportTranscript(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	svc := New(nil, paths)
	wantExports := paths.ExportsDir()
	if svc.ExportsDir() != wantExports {
		t.Errorf("ExportsDir() = %q, want %q", svc.ExportsDir(), wantExports)
	}

	s := agent.Session{AgentID: "claude-code", ID: "s1", Title: "t"}
	path, err := svc.ExportTranscript(s, []agent.Entry{{Role: "user", Text: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != wantExports {
		t.Errorf("export went to the wrong dir: %s", path)
	}
}

func TestExportRefusesTraversal(t *testing.T) {
	for _, id := range []string{"../escape", "x/../../escape", `x\escape`} {
		if _, err := ExportMarkdown(agent.Session{AgentID: "codex", ID: id}, []agent.Entry{{Text: "private"}}, t.TempDir()); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
}
