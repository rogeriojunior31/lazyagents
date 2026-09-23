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
		Title: "minha sessão", MTime: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC),
	}
	entries := []agent.Entry{
		{Role: "user", Text: "como faço X?"},
		{Role: "assistant", Text: "faça Y."},
		{Role: agent.RoleTool, Text: "Bash · go test ./..."},
		{Role: agent.RoleThinking, Text: "linha 1\nlinha 2"},
	}

	path, err := ExportMarkdown(s, entries, dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("path fora do dir esperado: %s", path)
	}
	if !strings.HasPrefix(filepath.Base(path), "claude-code-sess-1-") || !strings.HasSuffix(path, ".md") {
		t.Errorf("nome de arquivo inesperado: %s", filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"minha sessão", "Claude Code", "/tmp/proj", "▶ você", "como faço X?", "◀ Claude Code", "faça Y.", "- ❯ `Bash · go test ./...`", "> 💭 linha 1\n> linha 2"} {
		if !strings.Contains(content, want) {
			t.Errorf("export não contém %q:\n%s", want, content)
		}
	}

	// transcript vazio: erro, sem gravar arquivo
	before, _ := os.ReadDir(dir)
	if _, err := ExportMarkdown(s, nil, dir); err == nil {
		t.Fatal("transcript vazio deveria ser erro")
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before) {
		t.Fatal("transcript vazio não deveria gravar arquivo")
	}
}

func TestServiceExportTranscript(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	svc := New(nil, paths)
	wantExports := paths.ExportsDir()
	if svc.ExportsDir() != wantExports {
		t.Errorf("ExportsDir() = %q, quer %q", svc.ExportsDir(), wantExports)
	}

	s := agent.Session{AgentID: "claude-code", ID: "s1", Title: "t"}
	path, err := svc.ExportTranscript(s, []agent.Entry{{Role: "user", Text: "oi"}})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != wantExports {
		t.Errorf("export foi pro dir errado: %s", path)
	}
}

func TestExportRefusesTraversal(t *testing.T) {
	for _, id := range []string{"../escape", "x/../../escape", `x\escape`} {
		if _, err := ExportMarkdown(agent.Session{AgentID: "codex", ID: id}, []agent.Entry{{Text: "private"}}, t.TempDir()); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
}
