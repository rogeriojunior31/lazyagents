package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClaudeProviderApplyReadClear(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	path := c.ProviderFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	const original = `{
  "model": "opus",
  "env": {"MY_VAR": "1"},
  "hooks": {"SessionStart": []}
}
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := c.ReadProvider(); ok || err != nil {
		t.Fatalf("ReadProvider with no provider = %v, %v; want false, nil", ok, err)
	}

	p := ProviderProfile{Name: "mine", BaseURL: "https://example/api", Token: "secret-abc", Model: "sonnet"}
	backups := filepath.Join(home, "backups")
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"hooks"`, `"MY_VAR"`, "https://example/api", "secret-abc"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("settings.json lacks %q:\n%s", want, data)
		}
	}
	if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	got, ok, err := c.ReadProvider()
	if err != nil || !ok {
		t.Fatalf("ReadProvider = %v, %v", ok, err)
	}
	if got.BaseURL != p.BaseURL || got.Model != p.Model {
		t.Errorf("ReadProvider = %+v", got)
	}
	if got.Token != "" || !got.HasToken {
		t.Errorf("token leaked on read: %+v", got)
	}

	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-abc") || strings.Contains(string(data), "example") {
		t.Errorf("clear did not clean up:\n%s", data)
	}
	for _, want := range []string{`"hooks"`, `"MY_VAR"`, `"model"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("clear dropped %q:\n%s", want, data)
		}
	}
	if _, ok, _ := c.ReadProvider(); ok {
		t.Error("ReadProvider after clear should be false")
	}
}

const codexLiveTOML = `model = "gpt-6"
model_reasoning_effort = "medium"

[projects."/home/me/Projects"]
trust_level = "trusted"

[hooks.state."/home/me/.codex/hooks.json:session_start:0:0"]
trusted_hash = "sha256:abc"
`

func TestCodexProviderApplyPreservesFileAndClears(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	path := c.ProviderFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(codexLiveTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	p := ProviderProfile{Name: "local", BaseURL: "http://localhost:11434/v1", EnvKey: "MY_KEY", Model: "qwen", WireAPI: "responses"}
	backups := filepath.Join(home, "backups")
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`trusted_hash = "sha256:abc"`, `[projects."/home/me/Projects"]`, `model_reasoning_effort = "medium"`} {
		if !strings.Contains(text, want) {
			t.Errorf("apply dropped %q:\n%s", want, text)
		}
	}
	// the top-level key must go BEFORE the first table, or it belongs to the
	// previous table
	if i, j := strings.Index(text, "model_provider ="), strings.Index(text, "[projects."); i < 0 || i > j {
		t.Errorf("model_provider not at the top:\n%s", text)
	}
	if !strings.Contains(text, "[model_providers.lazyagents]") || !strings.Contains(text, `env_key = "MY_KEY"`) {
		t.Errorf("provider table missing:\n%s", text)
	}

	got, ok, err := c.ReadProvider()
	if err != nil || !ok {
		t.Fatalf("ReadProvider = %v, %v", ok, err)
	}
	if got.BaseURL != p.BaseURL || got.Model != "qwen" || got.EnvKey != "MY_KEY" || got.WireAPI != "responses" || got.Name != "local" {
		t.Errorf("ReadProvider = %+v", got)
	}

	// while the profile is applied, its model is the only one at the root
	if n := strings.Count(text, "\nmodel = "); n != 1 {
		t.Errorf("model key repeated at the root (%d):\n%s", n, text)
	}
	if !strings.Contains(text, `model = "qwen"`) || !strings.Contains(text, codexPrevModel+`"gpt-6"`) {
		t.Errorf("model swap did not keep the previous one:\n%s", text)
	}

	// reapplying does not duplicate the block
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if n := strings.Count(string(data), "[model_providers.lazyagents]"); n != 1 {
		t.Errorf("duplicate blocks: %d\n%s", n, data)
	}

	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	text = string(data)
	if strings.Contains(text, "lazyagents") {
		t.Errorf("clear left a managed block:\n%s", text)
	}
	// the user's model comes back exactly once (a repeated key breaks the TOML)
	if n := strings.Count(text, `model = "gpt-6"`); n != 1 {
		t.Errorf("original model restored %d time(s):\n%s", n, text)
	}
	for _, want := range []string{`trusted_hash = "sha256:abc"`, `model_reasoning_effort = "medium"`, `[projects."/home/me/Projects"]`} {
		if !strings.Contains(text, want) {
			t.Errorf("clear dropped %q:\n%s", want, text)
		}
	}
	if _, ok, _ := c.ReadProvider(); ok {
		t.Error("ReadProvider after clear should be false")
	}
}

func TestCodexProviderRefusesTokenWithoutEnvKey(t *testing.T) {
	c := NewCodex(t.TempDir())
	err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "https://e", Token: "secret"}, "")
	if err == nil {
		t.Fatal("want an error: Codex does not keep tokens in config.toml")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("token leaked in the error: %v", err)
	}
	if err := c.ApplyProvider(ProviderProfile{Name: "x"}, ""); err == nil {
		t.Error("want an error without baseUrl")
	}
}

func TestCodexProviderReadsHandSetProvider(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	const manual = `model_provider = "ollama"
model = "qwen3"

[model_providers.ollama]
name = "Ollama"
base_url = "http://localhost:11434/v1"  # comment
`
	if err := os.WriteFile(c.ProviderFile(), []byte(manual), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.ReadProvider()
	if err != nil || !ok {
		t.Fatalf("ReadProvider = %v, %v", ok, err)
	}
	if got.Name != "Ollama" || got.BaseURL != "http://localhost:11434/v1" || got.Model != "qwen3" {
		t.Errorf("ReadProvider = %+v", got)
	}
}

func TestProviderProfileRedacted(t *testing.T) {
	p := ProviderProfile{Name: "x", Token: "secret"}.Redacted()
	if p.Token != "" || !p.HasToken {
		t.Errorf("Redacted = %+v", p)
	}
	if again := p.Redacted(); !again.HasToken { // idempotent
		t.Errorf("Redacted twice lost HasToken: %+v", again)
	}
	got := ProviderProfile{Name: "x"}.Redacted()
	if got.HasToken {
		t.Errorf("HasToken without a token: %+v", got)
	}
}

func TestCodexPreservesPreviousProvider(t *testing.T) {
	c := NewCodex(t.TempDir())
	original := "model_provider = 'other'\nmodel = \"original\"\n[model_providers.other]\nname = \"Other\"\nbase_url = \"https://example.com\"\n"
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	p := ProviderProfile{Name: "new", BaseURL: "https://example.org", Model: "replacement"}
	if err := c.ApplyProvider(p, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if strings.Count(string(data), "model_provider =") != 1 {
		t.Fatalf("duplicate provider: %s", data)
	}
	if err := c.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(c.ProviderFile())
	top, _ := parseCodexTOML(string(data))
	if top["model_provider"] != "other" || top["model"] != "original" {
		t.Fatalf("not restored: %s", data)
	}
}

func TestCodexRefusesUnclosedManagedBlock(t *testing.T) {
	c := NewCodex(t.TempDir())
	original := codexBlockStart + "\nmodel = \"x\"\n[projects.mine]\ntrust_level = \"trusted\"\n"
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearProvider(""); err == nil {
		t.Fatal("accepted malformed block")
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if string(data) != original {
		t.Fatal("config changed")
	}
}

func TestClaudeProviderKeepsUnknownEnvValues(t *testing.T) {
	c := NewClaude(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(`{"env":{"future":{"nested":true}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://example.com"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if !strings.Contains(string(data), `"nested"`) {
		t.Fatalf("lost unknown field: %s", data)
	}
}

func TestClaudeTokenTightensPermissions(t *testing.T) {
	c := NewClaude(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{Token: "private"}, ""); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(c.ProviderFile())
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
}

func TestCodexRejectsUnsupportedWireAPI(t *testing.T) {
	if err := NewCodex(t.TempDir()).ApplyProvider(ProviderProfile{BaseURL: "https://example.com", WireAPI: "chat"}, ""); err == nil {
		t.Fatal("accepted removed protocol")
	}
}

func TestCodexRefusesExistingUnmanagedProviderTable(t *testing.T) {
	c := NewCodex(t.TempDir())
	original := "[model_providers.lazyagents] # owned by user\n"
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://example.com"}, ""); err == nil {
		t.Fatal("duplicated unmanaged table")
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if string(data) != original {
		t.Fatal("modified config")
	}
}

// A config.toml written before the English migration: legacy markers must
// still be recognized, and the next write must leave only English ones.
func TestCodexMigratesLegacyMarkers(t *testing.T) {
	legacy := strings.Join([]string{
		codexLegacyBlockStart,
		`model_provider = "lazyagents"`,
		`model = "qwen"`,
		codexLegacyPrevProvider + `"other"`,
		codexLegacyPrevModel + `"gpt-6"`,
		codexLegacyBlockEnd,
		"",
		"[projects.mine]",
		`trust_level = "trusted"`,
		"",
		codexLegacyBlockStart,
		"[model_providers.lazyagents]",
		`name = "old"`,
		`base_url = "https://old.example.com"`,
		codexLegacyBlockEnd,
		"",
	}, "\n")
	write := func(t *testing.T) *Codex {
		t.Helper()
		c := NewCodex(t.TempDir())
		if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(c.ProviderFile(), []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
		return c
	}
	noLegacy := func(t *testing.T, text string) {
		t.Helper()
		for _, m := range []string{codexLegacyBlockStart, codexLegacyBlockEnd, codexLegacyPrevModel, codexLegacyPrevProvider} {
			if strings.Contains(text, m) {
				t.Fatalf("legacy marker %q left behind:\n%s", m, text)
			}
		}
		if !strings.Contains(text, "[projects.mine]\ntrust_level = \"trusted\"") {
			t.Fatalf("foreign table lost:\n%s", text)
		}
	}

	t.Run("read", func(t *testing.T) {
		p, ok, err := write(t).ReadProvider()
		if err != nil || !ok || p.Name != "old" || p.BaseURL != "https://old.example.com" || p.Model != "qwen" {
			t.Fatalf("got %+v ok=%v err=%v", p, ok, err)
		}
	})
	t.Run("apply", func(t *testing.T) {
		c := write(t)
		if err := c.ApplyProvider(ProviderProfile{Name: "new", BaseURL: "https://new.example.com", Model: "kimi"}, ""); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(c.ProviderFile())
		text := string(data)
		noLegacy(t, text)
		for _, want := range []string{codexBlockStart, codexBlockEnd, codexPrevProvider + `"other"`, codexPrevModel + `"gpt-6"`, `base_url = "https://new.example.com"`} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q:\n%s", want, text)
			}
		}
		if strings.Count(text, "model_provider =") != 1 || strings.Count(text, "\nmodel =") != 1 {
			t.Fatalf("duplicate keys:\n%s", text)
		}
	})
	t.Run("clear", func(t *testing.T) {
		c := write(t)
		if err := c.ClearProvider(""); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(c.ProviderFile())
		text := string(data)
		noLegacy(t, text)
		if strings.Contains(text, "lazyagents") {
			t.Fatalf("managed block left behind:\n%s", text)
		}
		top, _ := parseCodexTOML(text)
		if top["model_provider"] != "other" || top["model"] != "gpt-6" {
			t.Fatalf("user values not restored:\n%s", text)
		}
	})
}

// writeClaudeSettings creates a Claude adapter whose provider state persists
// in the temp dir, as in production, with settings.json holding content.
func writeClaudeSettings(t *testing.T, content string) (*Claude, string) {
	t.Helper()
	home := t.TempDir()
	c := NewClaude(home)
	c.ProviderState = filepath.Join(home, "data", "claude-provider-state.json")
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return c, filepath.Join(home, "backups")
}

// fresh returns a new adapter on the same files: each CLI command is a new process.
func fresh(c *Claude) *Claude {
	n := NewClaude(c.Home)
	n.ProviderState = c.ProviderState
	return n
}

func TestClaudeHandSetTokenSurvivesApplyAndClear(t *testing.T) {
	c, backups := writeClaudeSettings(t, `{"env":{"ANTHROPIC_AUTH_TOKEN":"mine"}}`)
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://proxy.example"}, backups); err != nil {
		t.Fatal(err)
	}
	if err := fresh(c).ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if !strings.Contains(string(data), `"ANTHROPIC_AUTH_TOKEN": "mine"`) {
		t.Fatalf("hand-set token lost:\n%s", data)
	}
	if strings.Contains(string(data), "proxy.example") {
		t.Fatalf("clear left the base URL:\n%s", data)
	}
}

func TestClaudeSwitchDropsPreviousProfileModel(t *testing.T) {
	c, backups := writeClaudeSettings(t, `{}`)
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://a.example", Model: "model-a"}, backups); err != nil {
		t.Fatal(err)
	}
	if err := fresh(c).ApplyProvider(ProviderProfile{BaseURL: "https://b.example"}, backups); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.ReadProvider()
	if err != nil || !ok || got.Model != "" || got.BaseURL != "https://b.example" {
		t.Fatalf("after switch = %+v, %v, %v; want B without A's model", got, ok, err)
	}
}

func TestClaudeApplyClearRestoresFileByteForByte(t *testing.T) {
	const original = `{
  "model": "opus",
  "env": {
    "ZED": "1",
    "ANTHROPIC_MODEL": "hand-picked",
    "ALPHA": "2"
  },
  "hooks": {}
}
`
	c, backups := writeClaudeSettings(t, original)
	p := ProviderProfile{BaseURL: "https://proxy.example", Token: "secret", Model: "sonnet"}
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	// env keeps its order: existing keys in place, new ones appended.
	if i, j, k := strings.Index(string(data), `"ZED"`), strings.Index(string(data), `"ANTHROPIC_MODEL": "sonnet"`), strings.Index(string(data), `"ALPHA"`); !(i < j && j < k) {
		t.Fatalf("env order changed:\n%s", data)
	}
	state, _ := os.ReadFile(c.ProviderState)
	if strings.Contains(string(state), "secret") {
		t.Fatalf("token written to the state file:\n%s", state)
	}
	if err := fresh(c).ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(c.ProviderFile())
	if string(data) != original {
		t.Fatalf("apply→clear changed the file:\n%s\nwant:\n%s", data, original)
	}
}

func TestClaudeEditedValueIsNotRemoved(t *testing.T) {
	c, backups := writeClaudeSettings(t, `{}`)
	if err := c.ApplyProvider(ProviderProfile{Model: "sonnet"}, backups); err != nil {
		t.Fatal(err)
	}
	// the user changes the model by hand after the apply: it is theirs now
	if err := os.WriteFile(c.ProviderFile(), []byte(`{"env":{"ANTHROPIC_MODEL":"opus"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fresh(c).ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := c.ReadProvider(); got.Model != "opus" {
		t.Fatalf("clear removed a value the user edited: %+v", got)
	}
}

func TestClaudeClearWithoutRecordKeepsUserKeys(t *testing.T) {
	// No record: the keys may be the user's own proxy setup, so clear leaves them.
	const orig = `{"env":{"ANTHROPIC_BASE_URL":"https://mine.example","ANTHROPIC_MODEL":"m","MY_VAR":"1"}}`
	c, backups := writeClaudeSettings(t, orig)
	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if !strings.Contains(string(data), "mine.example") || !strings.Contains(string(data), `"m"`) {
		t.Fatalf("clear without a record removed user keys:\n%s", data)
	}
}

// The record is written before settings.json: when it cannot be saved, the
// live file stays untouched instead of holding values nobody recorded.
func TestClaudeApplyKeepsSettingsWhenStateFails(t *testing.T) {
	const orig = `{"env":{"MY_VAR":"1"}}`
	c, backups := writeClaudeSettings(t, orig)
	blocker := filepath.Dir(c.ProviderState)
	if err := os.WriteFile(blocker, []byte("a file, not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://p.example"}, backups); err == nil {
		t.Fatal("apply should fail when the record cannot be written")
	}
	if data, _ := os.ReadFile(c.ProviderFile()); string(data) != orig {
		t.Errorf("settings.json changed without a record:\n%s", data)
	}
}

// A settings.json that lazyagents created only for the provider is removed on
// clear; one that also got other content stays.
func TestClaudeClearRemovesFileItCreated(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	c.ProviderState = filepath.Join(home, "data", "claude-provider-state.json")
	backups := filepath.Join(home, "backups")
	p := ProviderProfile{BaseURL: "https://p.example", Model: "m"}
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.ProviderFile()); !os.IsNotExist(err) {
		data, _ := os.ReadFile(c.ProviderFile())
		t.Fatalf("settings.json should be gone, got:\n%s", data)
	}

	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	if err := c.AddHook(Hook{Event: HookStop, Command: "true"}, backups); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(c.ProviderFile()); err != nil || !strings.Contains(string(data), "hooks") {
		t.Errorf("a file with hooks must stay: %s %v", data, err)
	}
}
