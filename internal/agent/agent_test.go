package agent

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// noBin simulates a missing binary so detection is deterministic.
func noBin(string) (string, error) { return "", errors.New("not found") }

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
		ad                  Adapter
		id, managed, shared string
	}{
		{&Claude{Home: home, Look: noBin}, "claude-code", ".claude/skills", ""},
		{&Codex{Home: home, Look: noBin}, "codex", ".codex/skills", ".agents/skills"},
		{&Gemini{Home: home, Look: noBin}, "gemini-cli", ".gemini/skills", ".agents/skills"},
		{&OpenCode{Home: home, Look: noBin}, "opencode", ".config/opencode/skills", ".agents/skills"},
	}
	for _, tc := range tests {
		a := tc.ad.Detect()
		if a.ID != tc.id || tc.ad.ID() != tc.id {
			t.Errorf("ID = %s/%s, want %s", a.ID, tc.ad.ID(), tc.id)
		}
		if !a.Installed {
			t.Errorf("%s: should be installed (config dir exists)", tc.id)
		}
		if want := filepath.Join(home, tc.managed); a.ManagedDir != want {
			t.Errorf("%s: ManagedDir = %s, want %s", tc.id, a.ManagedDir, want)
		}
		if len(a.ReadDirs) == 0 || a.ReadDirs[0] != a.ManagedDir {
			t.Errorf("%s: ReadDirs[0] must be ManagedDir: %v", tc.id, a.ReadDirs)
		}
		if tc.shared != "" && a.SharedDir != filepath.Join(home, tc.shared) || tc.shared == "" && a.SharedDir != "" {
			t.Errorf("%s: SharedDir = %q, want %q", tc.id, a.SharedDir, tc.shared)
		}
	}
	// empty home and no binary → not installed
	empty := t.TempDir()
	if a := (&Claude{Home: empty, Look: noBin}).Detect(); a.Installed {
		t.Error("claude should not be detected in an empty home")
	}
	if a := (&ClaudeDesktop{Home: empty, Look: noBin}).Detect(); a.Installed || a.SupportsSkills() {
		t.Errorf("claude-desktop: %+v", a)
	}
	if a := (&Hermes{Home: empty, Look: noBin}).Detect(); a.Installed {
		t.Error("hermes should not be detected in an empty home")
	}
}

func TestRegistry(t *testing.T) {
	adapters := All(t.TempDir())
	if len(adapters) != 8 {
		t.Fatalf("All = %d adapters, want 8", len(adapters))
	}
	for _, id := range []string{"claude-code", "codex", "gemini-cli", "opencode", "claude-desktop", "hermes-agent", "pi", "crush"} {
		if ByID(adapters, id) == nil {
			t.Errorf("ByID(%s) = nil", id)
		}
	}
	if ByID(adapters, "nope") != nil {
		t.Error("ByID of an unknown id should be nil")
	}
	if got := DetectAll(adapters); len(got) != 8 || got[0].ID != "claude-code" {
		t.Errorf("DetectAll out of order or incomplete: %d", len(got))
	}
}

func TestClaudeSessions(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "-tmp-proj")
	writeFile(t, filepath.Join(proj, "aaaa-1111.jsonl"),
		`{"type":"mode","mode":"normal","sessionId":"aaaa-1111"}
{"type":"user","isMeta":true,"message":{"role":"user","content":"<local-command>ignore</local-command>"},"cwd":"/tmp/x"}
{"type":"user","message":{"role":"user","content":"My  prompt\nreal"},"cwd":"/tmp/x"}
`)
	writeFile(t, filepath.Join(proj, "bbbb-2222.jsonl"),
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"In blocks"}]},"cwd":"/tmp/y"}
`)
	// renamed session: the LAST ai-title line beats the first prompt
	writeFile(t, filepath.Join(proj, "cccc-3333.jsonl"),
		`{"type":"user","message":{"role":"user","content":"original prompt"},"cwd":"/tmp/z"}
{"type":"ai-title","aiTitle":"old-name","sessionId":"cccc-3333"}
{"type":"ai-title","aiTitle":"renamed-name","sessionId":"cccc-3333"}
`)
	// the subagents subdir is ignored
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
		t.Fatalf("sessions = %d, want 3", len(got))
	}
	byID := map[string]Session{}
	for _, s := range got {
		byID[s.ID] = s
	}
	if s := byID["aaaa-1111"]; s.Title != "My prompt real" || s.CWD != "/tmp/x" {
		t.Errorf("session aaaa: %+v", s)
	}
	if s := byID["bbbb-2222"]; s.Title != "In blocks" || s.CWD != "/tmp/y" {
		t.Errorf("session bbbb: %+v", s)
	}
	if s := byID["cccc-3333"]; s.Title != "renamed-name" || s.CWD != "/tmp/z" {
		t.Errorf("renamed session: %+v", s)
	}
	if got[0].ID != "aaaa-1111" {
		t.Errorf("mtime order: first = %s", got[0].ID)
	}

	argv, dir, ok := c.ResumeCmd(Session{ID: "aaaa-1111", CWD: home})
	if !ok || dir != home || argv[0] != "claude" || argv[1] != "--resume" || argv[2] != "aaaa-1111" {
		t.Errorf("resume: %v %s %v", argv, dir, ok)
	}
	// missing cwd → home fallback
	if _, dir, _ := c.ResumeCmd(Session{ID: "x", CWD: "/does/not/exist"}); dir != home {
		t.Errorf("cwd fallback: %s", dir)
	}
}

func TestClaudeSessionUsage(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "-tmp-proj", "aaaa-1111.jsonl")
	writeFile(t, path,
		`{"type":"user","message":{"role":"user","content":"hi"},"cwd":"/tmp/x"}
{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"a"}],"usage":{"input_tokens":100,"output_tokens":200,"cache_read_input_tokens":50,"cache_creation_input_tokens":25}}}
this line is not valid json and must be skipped without breaking
{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"b"}],"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`)
	c := &Claude{Home: home, Look: noBin}
	u, ok := c.SessionUsage(Session{Path: path})
	if !ok {
		t.Fatal("expected ok=true with usage present")
	}
	if u.Input != 110 || u.Output != 220 || u.CacheRead != 50 || u.CacheWrite != 25 {
		t.Fatalf("wrong sum: %+v", u)
	}
	if u.Model != "claude-sonnet-4-5-20250929" {
		t.Fatalf("wrong model: %q", u.Model)
	}

	// session with no assistant usage line: ok=false, no crash
	noUsagePath := filepath.Join(home, ".claude", "projects", "-tmp-proj", "no-usage.jsonl")
	writeFile(t, noUsagePath, `{"type":"user","message":{"role":"user","content":"hi"}}`+"\n")
	if _, ok := c.SessionUsage(Session{Path: noUsagePath}); ok {
		t.Fatal("a session without usage should be ok=false")
	}

	// missing file: no crash
	if _, ok := c.SessionUsage(Session{Path: filepath.Join(home, "missing.jsonl")}); ok {
		t.Fatal("a missing file should be ok=false")
	}
}

func TestClaudeIsLive(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available here")
	}
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "-tmp-proj")
	path := filepath.Join(proj, "aaaa-1111.jsonl")
	writeFile(t, path, `{"type":"user","message":{"role":"user","content":"hi"},"cwd":"/tmp/x"}`+"\n")
	other := filepath.Join(proj, "bbbb-2222.jsonl")
	writeFile(t, other, `{"type":"user","message":{"role":"user","content":"hi"},"cwd":"/tmp/x"}`+"\n")

	c := &Claude{Home: home, Look: noBin}
	sessions, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	var s1, s2 Session
	for _, s := range sessions {
		switch s.ID {
		case "aaaa-1111":
			s1 = s
		case "bbbb-2222":
			s2 = s
		}
	}
	// no process has the file open: neither is live
	if c.IsLive(s1) || c.IsLive(s2) {
		t.Fatal("with no open process, IsLive should be false for both")
	}

	// really open s1 in this process: lsof must see the test itself
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sessions, err = c.ListSessions() // recomputes the cache with the file open
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.ID == "aaaa-1111" {
			s1 = s
		}
	}
	if !c.IsLive(s1) {
		t.Error("aaaa-1111, opened by the test itself, should be live")
	}
	if c.IsLive(s2) {
		t.Error("bbbb-2222 was never opened and should not be live")
	}

	// IsLive before any ListSessions (new adapter): always false
	if (&Claude{Home: home, Look: noBin}).IsLive(s1) {
		t.Error("without a prior ListSessions, IsLive should be false")
	}
}

func TestCodexSessions(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, ".codex", "sessions", "2026", "07", "06")
	writeFile(t, filepath.Join(base, "rollout-2026-07-06T12-00-00-1234.jsonl"),
		`{"type":"session_meta","payload":{"id":"sess-abc","cwd":"/tmp/w"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hi codex"}]}}
`)
	// no session_meta → ID = last 36 chars of the name
	writeFile(t, filepath.Join(base, "rollout-2026-07-06T13-00-00-123e4567-e89b-12d3-a456-426614174000.jsonl"), `{}`)

	c := &Codex{Home: home, Look: noBin}
	got, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("sessions = %d, want 2", len(got))
	}
	byID := map[string]Session{}
	for _, s := range got {
		byID[s.ID] = s
	}
	meta := byID["sess-abc"]
	if meta.CWD != "/tmp/w" || meta.Title != "Hi codex" {
		t.Errorf("session with meta: %+v", meta)
	}
	if _, ok := byID["123e4567-e89b-12d3-a456-426614174000"]; !ok {
		t.Errorf("ID fallback from the file name failed: %v", byID)
	}
	argv, _, ok := c.ResumeCmd(Session{ID: "sess-abc"})
	if !ok || argv[0] != "codex" || argv[1] != "resume" {
		t.Errorf("resume: %v", argv)
	}
}

func TestGeminiSessions(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".gemini", "projects.json"),
		`{"projects":{"/tmp/proj":"myproj"}}`)
	line := `{"sessionId":"abc-123","projectHash":"x","startTime":"2026-07-06T10:00:00Z","kind":"main"}
{"role":"user","parts":[{"text":"Hi gemini"}]}
`
	writeFile(t, filepath.Join(home, ".gemini", "history", "myproj", "chats", "session-1.jsonl"), line)
	// a duplicate in tmp/ is deduplicated
	writeFile(t, filepath.Join(home, ".gemini", "tmp", "myproj", "chats", "session-1.jsonl"), line)

	g := &Gemini{Home: home, Look: noBin}
	got, err := g.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("sessions = %d, want 1 (dedupe)", len(got))
	}
	s := got[0]
	if s.ID != "abc-123" || s.CWD != "/tmp/proj" || s.Title != "Hi gemini" {
		t.Errorf("session: %+v", s)
	}
	argv, _, ok := g.ResumeCmd(s)
	if !ok || argv[0] != "gemini" || argv[1] != "--resume" || argv[2] != "abc-123" {
		t.Errorf("resume: %v", argv)
	}
}

func TestUtil(t *testing.T) {
	if got := cleanTitle("  a   b\n\tc  ", 80); got != "a b c" {
		t.Errorf("cleanTitle whitespace: %q", got)
	}
	if got := cleanTitle("<tag>x</tag>", 80); got != "" {
		t.Errorf("cleanTitle tag: %q", got)
	}
	if got := cleanTitle("abcdef", 4); got != "abc…" {
		t.Errorf("cleanTitle cut: %q", got)
	}
	if got := extractAnyText([]any{map[string]any{"type": "text", "text": "hi"}}); got != "hi" {
		t.Errorf("extractAnyText blocks: %q", got)
	}
	if got := looseUserText([]byte(`{"type":"user","message":{"role":"user","content":"plain"}}`)); got != "plain" {
		t.Errorf("looseUserText claude: %q", got)
	}
	if got := looseUserText([]byte(`{"isMeta":true,"role":"user","content":"x"}`)); got != "" {
		t.Errorf("looseUserText isMeta: %q", got)
	}
}
