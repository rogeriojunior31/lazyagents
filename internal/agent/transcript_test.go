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
{"type":"user","message":{"role":"user","content":"Pergunta um"}}
linha inválida que não é JSON
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Resposta **um**"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"ignorado"}]}}
{"type":"user","message":{"role":"user","content":"Pergunta dois"}}
`)
	c := &Claude{Home: home, Look: noBin}
	got, err := c.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: "user", Text: "Pergunta um"},
		{Role: "assistant", Text: "Resposta **um**"},
		{Role: "user", Text: "Pergunta dois"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %d, quer %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, quer %+v", i, got[i], want[i])
		}
	}
}

func TestCodexTranscript(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "r.jsonl")
	writeFile(t, path,
		`{"type":"session_meta","payload":{"id":"x","cwd":"/tmp"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Oi codex"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Olá!"}]}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<user_instructions>ignorar</user_instructions>"}]}}
`)
	c := &Codex{Home: home, Look: noBin}
	got, err := c.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "Oi codex" || got[1] != (Entry{Role: "assistant", Text: "Olá!"}) {
		t.Fatalf("entries: %+v", got)
	}
}

func TestGeminiTranscript(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gemini", "history", "p", "chats", "session-1.jsonl")
	writeFile(t, path,
		`{"sessionId":"abc","startTime":"2026-07-06T10:00:00Z"}
{"role":"user","parts":[{"text":"Oi gemini"}]}
{"role":"model","parts":[{"text":"Olá, humano"}]}
`)
	g := &Gemini{Home: home, Look: noBin}
	got, err := g.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "Oi gemini" || got[1] != (Entry{Role: "assistant", Text: "Olá, humano"}) {
		t.Fatalf("entries: %+v", got)
	}
}

func TestGeminiTranscriptSetWrapper(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gemini", "tmp", "p", "chats", "session-2.jsonl")
	writeFile(t, path,
		`{"sessionId":"abc"}
{"type":"user","content":[{"text":"direto"}]}
{"$set":{"messages":[{"type":"user","content":[{"text":"do lote"}]},{"type":"gemini","content":[{"text":"resposta do lote"}]}]}}
{"$set":{"lastUpdated":"2026-05-12T18:09:58.424Z"}}
`)
	g := &Gemini{Home: home, Look: noBin}
	got, err := g.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: "user", Text: "direto"},
		{Role: "user", Text: "do lote"},
		{Role: "assistant", Text: "resposta do lote"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %+v, quer %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, quer %+v", i, got[i], want[i])
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
		t.Fatalf("truncamento falhou: len=%d", len(got))
	}
}

func TestUnsupportedTranscript(t *testing.T) {
	if _, err := (&ClaudeDesktop{Home: t.TempDir(), Look: noBin}).Transcript(Session{}); err == nil {
		t.Error("claude-desktop deveria recusar transcript")
	}
	if _, err := (&Hermes{Home: t.TempDir(), Look: noBin}).Transcript(Session{}); err == nil {
		t.Error("hermes deveria recusar transcript")
	}
}

// Cada linha vira blocos na ordem em que o agente os produziu: raciocínio
// gravado em texto, fala e chamadas de ferramenta (uma linha cada), nos
// formatos de Claude, Codex, Gemini e OpenCode. Raciocínio vazio ou
// criptografado e resultados de ferramenta ficam de fora.
func TestTranscriptBlocks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "s.jsonl")
	writeFile(t, path,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"preciso ver os arquivos"},{"type":"text","text":"Vou listar."},{"type":"tool_use","name":"Bash","input":{"command":"ls   -la\n/tmp","description":"lista"}},{"type":"text","text":"Pronto."}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"abc"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"saída enorme"}]}}
{"type":"response_item","payload":{"type":"reasoning","summary":[],"encrypted_content":"gAAA"}}
{"type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"Checando os testes"}]}}
{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{\"command\":[\"bash\",\"-lc\",\"go test ./...\"]}"}}
{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","input":"text(await tools.exec_command({cmd:\"pwd && rg --files -g 'go.mod'\",yield:1}))"}}
{"type":"gemini","content":"Feito.","thoughts":[{"subject":"Plano","description":"rodar o build"}]}
{"role":"assistant","parts":[{"type":"reasoning","text":"opencode pensando"},{"type":"text","text":"ok"}]}
`)
	got, err := jsonlTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Role: RoleThinking, Text: "preciso ver os arquivos"},
		{Role: RoleAssistant, Text: "Vou listar."},
		{Role: RoleTool, Text: "Bash · ls -la /tmp"},
		{Role: RoleAssistant, Text: "Pronto."},
		{Role: RoleThinking, Text: "Checando os testes"},
		{Role: RoleTool, Text: "shell · bash -lc go test ./..."},
		{Role: RoleTool, Text: "exec · pwd && rg --files -g 'go.mod'"},
		{Role: RoleThinking, Text: "**Plano** rodar o build"},
		{Role: RoleAssistant, Text: "Feito."},
		{Role: RoleThinking, Text: "opencode pensando"},
		{Role: RoleAssistant, Text: "ok"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %d, quer %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, quer %+v", i, got[i], want[i])
		}
	}
}
