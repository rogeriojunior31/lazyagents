package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// noBin simula binário ausente para detecção determinística em teste.
func noBin(string) (string, error) { return "", errors.New("não encontrado") }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".gemini", ".config/opencode"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		ad          Adapter
		id, managed string
	}{
		{&Claude{Home: home, Look: noBin}, "claude-code", ".claude/skills"},
		{&Codex{Home: home, Look: noBin}, "codex", ".agents/skills"},
		{&Gemini{Home: home, Look: noBin}, "gemini-cli", ".gemini/skills"},
		{&OpenCode{Home: home, Look: noBin}, "opencode", ".config/opencode/skills"},
	}
	for _, tc := range tests {
		a := tc.ad.Detect()
		if a.ID != tc.id || tc.ad.ID() != tc.id {
			t.Errorf("ID = %s/%s, quer %s", a.ID, tc.ad.ID(), tc.id)
		}
		if !a.Installed {
			t.Errorf("%s: deveria estar instalado (dir de config existe)", tc.id)
		}
		if want := filepath.Join(home, tc.managed); a.ManagedDir != want {
			t.Errorf("%s: ManagedDir = %s, quer %s", tc.id, a.ManagedDir, want)
		}
		if len(a.ReadDirs) == 0 || a.ReadDirs[0] != a.ManagedDir {
			t.Errorf("%s: ReadDirs[0] deve ser o ManagedDir: %v", tc.id, a.ReadDirs)
		}
	}
	// home vazio + sem binário → não instalado
	empty := t.TempDir()
	if a := (&Claude{Home: empty, Look: noBin}).Detect(); a.Installed {
		t.Error("claude não deveria ser detectado em home vazio")
	}
	if a := (&ClaudeDesktop{Home: empty, Look: noBin}).Detect(); a.Installed || a.SupportsSkills() {
		t.Errorf("claude-desktop: %+v", a)
	}
	if a := (&Hermes{Home: empty, Look: noBin}).Detect(); a.Installed {
		t.Error("hermes não deveria ser detectado em home vazio")
	}
}

func TestRegistry(t *testing.T) {
	adapters := All(t.TempDir())
	if len(adapters) != 6 {
		t.Fatalf("All = %d adapters, quer 6", len(adapters))
	}
	for _, id := range []string{"claude-code", "codex", "gemini-cli", "opencode", "claude-desktop", "hermes-agent"} {
		if ByID(adapters, id) == nil {
			t.Errorf("ByID(%s) = nil", id)
		}
	}
	if ByID(adapters, "nope") != nil {
		t.Error("ByID de id desconhecido deveria ser nil")
	}
	if got := DetectAll(adapters); len(got) != 6 || got[0].ID != "claude-code" {
		t.Errorf("DetectAll fora de ordem ou incompleto: %d", len(got))
	}
}

func TestClaudeSessions(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "-tmp-proj")
	writeFile(t, filepath.Join(proj, "aaaa-1111.jsonl"),
		`{"type":"mode","mode":"normal","sessionId":"aaaa-1111"}
{"type":"user","isMeta":true,"message":{"role":"user","content":"<local-command>ignorar</local-command>"},"cwd":"/tmp/x"}
{"type":"user","message":{"role":"user","content":"Meu  prompt\nreal"},"cwd":"/tmp/x"}
`)
	writeFile(t, filepath.Join(proj, "bbbb-2222.jsonl"),
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Em blocos"}]},"cwd":"/tmp/y"}
`)
	// sessão renomeada: a ÚLTIMA linha ai-title vence o primeiro prompt
	writeFile(t, filepath.Join(proj, "cccc-3333.jsonl"),
		`{"type":"user","message":{"role":"user","content":"prompt original"},"cwd":"/tmp/z"}
{"type":"ai-title","aiTitle":"nome-antigo","sessionId":"cccc-3333"}
{"type":"ai-title","aiTitle":"nome-renomeado","sessionId":"cccc-3333"}
`)
	// subdir de subagents deve ser ignorado
	writeFile(t, filepath.Join(proj, "aaaa-1111", "subagents", "agent-x.jsonl"), `{}`)
	now := time.Now()
	os.Chtimes(filepath.Join(proj, "aaaa-1111.jsonl"), now, now)
	os.Chtimes(filepath.Join(proj, "bbbb-2222.jsonl"), now.Add(-time.Hour), now.Add(-time.Hour))
	os.Chtimes(filepath.Join(proj, "cccc-3333.jsonl"), now.Add(-2*time.Hour), now.Add(-2*time.Hour))

	c := &Claude{Home: home, Look: noBin}
	got, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("sessões = %d, quer 3", len(got))
	}
	byID := map[string]Session{}
	for _, s := range got {
		byID[s.ID] = s
	}
	if s := byID["aaaa-1111"]; s.Title != "Meu prompt real" || s.CWD != "/tmp/x" {
		t.Errorf("sessão aaaa: %+v", s)
	}
	if s := byID["bbbb-2222"]; s.Title != "Em blocos" || s.CWD != "/tmp/y" {
		t.Errorf("sessão bbbb: %+v", s)
	}
	if s := byID["cccc-3333"]; s.Title != "nome-renomeado" || s.CWD != "/tmp/z" {
		t.Errorf("sessão renomeada: %+v", s)
	}
	if got[0].ID != "aaaa-1111" {
		t.Errorf("ordenação por mtime: primeira = %s", got[0].ID)
	}

	argv, dir, ok := c.ResumeCmd(Session{ID: "aaaa-1111", CWD: home})
	if !ok || dir != home || argv[0] != "claude" || argv[1] != "--resume" || argv[2] != "aaaa-1111" {
		t.Errorf("resume: %v %s %v", argv, dir, ok)
	}
	// cwd inexistente → fallback home
	if _, dir, _ := c.ResumeCmd(Session{ID: "x", CWD: "/nao/existe"}); dir != home {
		t.Errorf("fallback de cwd: %s", dir)
	}
}

func TestClaudeSessionUsage(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "-tmp-proj", "aaaa-1111.jsonl")
	writeFile(t, path,
		`{"type":"user","message":{"role":"user","content":"oi"},"cwd":"/tmp/x"}
{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"a"}],"usage":{"input_tokens":100,"output_tokens":200,"cache_read_input_tokens":50,"cache_creation_input_tokens":25}}}
esta linha não é json válido, deve ser ignorada sem quebrar
{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"b"}],"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`)
	c := &Claude{Home: home, Look: noBin}
	u, ok := c.SessionUsage(Session{Path: path})
	if !ok {
		t.Fatal("esperava ok=true com usage presente")
	}
	if u.Input != 110 || u.Output != 220 || u.CacheRead != 50 || u.CacheWrite != 25 {
		t.Fatalf("soma errada: %+v", u)
	}
	if u.Model != "claude-sonnet-4-5-20250929" {
		t.Fatalf("modelo errado: %q", u.Model)
	}

	// sessão sem nenhuma linha assistant com usage: ok=false, não quebra
	noUsagePath := filepath.Join(home, ".claude", "projects", "-tmp-proj", "sem-usage.jsonl")
	writeFile(t, noUsagePath, `{"type":"user","message":{"role":"user","content":"oi"}}`+"\n")
	if _, ok := c.SessionUsage(Session{Path: noUsagePath}); ok {
		t.Fatal("sessão sem usage deveria ser ok=false")
	}

	// arquivo inexistente: não quebra
	if _, ok := c.SessionUsage(Session{Path: filepath.Join(home, "nao-existe.jsonl")}); ok {
		t.Fatal("arquivo inexistente deveria ser ok=false")
	}
}

func TestCodexSessions(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, ".codex", "sessions", "2026", "07", "06")
	writeFile(t, filepath.Join(base, "rollout-2026-07-06T12-00-00-1234.jsonl"),
		`{"type":"session_meta","payload":{"id":"sess-abc","cwd":"/tmp/w"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Oi codex"}]}}
`)
	// sem session_meta → ID = últimos 36 chars do nome
	writeFile(t, filepath.Join(base, "rollout-2026-07-06T13-00-00-123e4567-e89b-12d3-a456-426614174000.jsonl"), `{}`)

	c := &Codex{Home: home, Look: noBin}
	got, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("sessões = %d, quer 2", len(got))
	}
	byID := map[string]Session{}
	for _, s := range got {
		byID[s.ID] = s
	}
	meta := byID["sess-abc"]
	if meta.CWD != "/tmp/w" || meta.Title != "Oi codex" {
		t.Errorf("sessão com meta: %+v", meta)
	}
	if _, ok := byID["123e4567-e89b-12d3-a456-426614174000"]; !ok {
		t.Errorf("fallback de ID por filename falhou: %v", byID)
	}
	argv, _, ok := c.ResumeCmd(Session{ID: "sess-abc"})
	if !ok || argv[0] != "codex" || argv[1] != "resume" {
		t.Errorf("resume: %v", argv)
	}
}

func TestGeminiSessions(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".gemini", "projects.json"),
		`{"projects":{"/tmp/proj":"meuproj"}}`)
	line := `{"sessionId":"abc-123","projectHash":"x","startTime":"2026-07-06T10:00:00Z","kind":"main"}
{"role":"user","parts":[{"text":"Oi gemini"}]}
`
	writeFile(t, filepath.Join(home, ".gemini", "history", "meuproj", "chats", "session-1.jsonl"), line)
	// duplicata em tmp/ deve ser deduplicada
	writeFile(t, filepath.Join(home, ".gemini", "tmp", "meuproj", "chats", "session-1.jsonl"), line)

	g := &Gemini{Home: home, Look: noBin}
	got, err := g.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("sessões = %d, quer 1 (dedupe)", len(got))
	}
	s := got[0]
	if s.ID != "abc-123" || s.CWD != "/tmp/proj" || s.Title != "Oi gemini" {
		t.Errorf("sessão: %+v", s)
	}
	argv, _, ok := g.ResumeCmd(s)
	if !ok || argv[0] != "gemini" || argv[1] != "--resume" || argv[2] != "abc-123" {
		t.Errorf("resume: %v", argv)
	}
}

func TestUtil(t *testing.T) {
	if got := cleanTitle("  a   b\n\tc  ", 80); got != "a b c" {
		t.Errorf("cleanTitle espaços: %q", got)
	}
	if got := cleanTitle("<tag>x</tag>", 80); got != "" {
		t.Errorf("cleanTitle tag: %q", got)
	}
	if got := cleanTitle("abcdef", 4); got != "abc…" {
		t.Errorf("cleanTitle corte: %q", got)
	}
	if got := extractAnyText([]any{map[string]any{"type": "text", "text": "oi"}}); got != "oi" {
		t.Errorf("extractAnyText blocos: %q", got)
	}
	if got := looseUserText([]byte(`{"type":"user","message":{"role":"user","content":"direto"}}`)); got != "direto" {
		t.Errorf("looseUserText claude: %q", got)
	}
	if got := looseUserText([]byte(`{"isMeta":true,"role":"user","content":"x"}`)); got != "" {
		t.Errorf("looseUserText isMeta: %q", got)
	}
}
