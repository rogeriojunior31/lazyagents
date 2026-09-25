package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A live file with a user hook, a field lazyagents does not know and an entry
// of unknown type: none of it may be lost.
const liveHooks = `{
  "model": "opus",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "^(startup|resume)$",
        "enabled": true,
        "hooks": [
          {"type": "command", "command": "bash mine.sh", "timeout": 10}
        ]
      }
    ],
    "PreToolUse": [
      {"hooks": [{"type": "mcp", "command": "nothing"}]}
    ]
  }
}
`

func writeLive(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(liveHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeHooksAddRemovePreservesForeign(t *testing.T) {
	home := t.TempDir()
	path := writeLive(t, home)
	c := NewClaude(home)
	backups := filepath.Join(home, "backups")

	got, err := c.ReadHooks()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Command != "bash mine.sh" || got[0].Matcher != "^(startup|resume)$" || got[0].Timeout != 10 {
		t.Fatalf("ReadHooks = %+v", got) // the "mcp" entry is not a command: skipped on read
	}

	mine := Hook{Event: HookSessionStart, Command: "lazyagents doctor", Timeout: 5}
	if err := c.AddHook(mine, backups); err != nil {
		t.Fatal(err)
	}
	if err := c.AddHook(mine, backups); err != nil { // idempotent
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if n := strings.Count(text, "lazyagents doctor"); n != 1 {
		t.Errorf("hook installed %d time(s):\n%s", n, text)
	}
	for _, want := range []string{`"model": "opus"`, `"enabled": true`, `"mcp"`, "bash mine.sh"} {
		if !strings.Contains(text, want) {
			t.Errorf("add dropped %q:\n%s", want, text)
		}
	}
	if !json.Valid(data) {
		t.Fatalf("invalid JSON:\n%s", text)
	}

	if err := c.RemoveHook(mine, backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "lazyagents doctor") {
		t.Errorf("remove did not remove it:\n%s", data)
	}
	// back to the original content (the primitive normalizes formatting, so the
	// comparison is semantic)
	var before, after any
	_ = json.Unmarshal([]byte(liveHooks), &before)
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("add+remove did not restore the original file:\n%s", data)
	}
}

// Removing an event's last hook removes the event; removing the very last one
// removes the "hooks" key.
func TestHooksCleanupWhenEmpty(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	h := Hook{Event: HookStop, Command: "echo done"}
	if err := c.AddHook(h, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(c.HooksFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "echo done") {
		t.Fatalf("hook was not written:\n%s", data)
	}
	if err := c.RemoveHook(h, ""); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(c.HooksFile())
	if strings.Contains(string(data), "hooks") {
		t.Errorf("the hooks key should be gone:\n%s", data)
	}
}

// A lazyagents hook in a group the user shares with another command: only ours
// is removed.
func TestRemoveFromSharedGroup(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	shared := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"theirs.sh"},{"type":"command","command":"mine.sh"}]}]}}`
	if err := os.WriteFile(path, []byte(shared), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewClaude(home)
	if err := c.RemoveHook(Hook{Event: HookStop, Command: "mine.sh"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "mine.sh") || !strings.Contains(string(data), "theirs.sh") {
		t.Errorf("wrong removal from a shared group:\n%s", data)
	}
}

func TestCodexHooksFileAndNote(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	if filepath.Base(c.HooksFile()) != "hooks.json" {
		t.Errorf("HooksFile = %s", c.HooksFile())
	}
	// no config.toml: the feature is off
	if note := c.HooksNote(); !strings.Contains(note, "hooks are off") {
		t.Errorf("note = %q", note)
	}
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte("[features]\nhooks = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if note := c.HooksNote(); !strings.Contains(note, "trust") {
		t.Errorf("with hooks on, the note should mention trust: %q", note)
	}

	h := Hook{Event: HookSessionStart, Command: "echo hi", Timeout: 5}
	if err := c.AddHook(h, ""); err != nil {
		t.Fatal(err)
	}
	got, err := c.ReadHooks()
	if err != nil || len(got) != 1 || !got[0].Same(h) {
		t.Fatalf("ReadHooks = %+v, %v", got, err)
	}
	// lazyagents neither touches the trust state nor turns the feature on.
	cfg, _ := os.ReadFile(c.ProviderFile())
	if strings.Contains(string(cfg), "trusted_hash") {
		t.Errorf("config.toml was touched:\n%s", cfg)
	}
}

// An event written in another case must not become a duplicate key.
func TestEventKeyIsCaseInsensitive(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	if err := os.MkdirAll(c.configDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.HooksFile(), []byte(`{"hooks":{"session_start":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.AddHook(Hook{Event: HookSessionStart, Command: "x.sh"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.HooksFile())
	if strings.Contains(string(data), "SessionStart") {
		t.Errorf("created a duplicate key:\n%s", data)
	}
	got, _ := c.ReadHooks()
	if len(got) != 1 {
		t.Errorf("ReadHooks = %+v", got)
	}
}
