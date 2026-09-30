package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Whatever a value holds, it goes back out of shQuote/shWords unchanged, so
// nothing lazyagents writes to crushrc can run as Bash.
func TestShQuoteRoundTrip(t *testing.T) {
	for _, v := range []string{"plain", "it's", `"$(rm -rf ~)"`, "a b\tc", `back\slash`, "'; touch /tmp/x; '", ""} {
		w := shWords("x " + shQuote(v))
		if len(w) != 2 || w[1] != v {
			t.Errorf("%q came back as %q", v, w)
		}
	}
	if w := shWords(`provider add id --api-key "$KEY"`); len(w) != 5 || w[4] != "$KEY" {
		t.Errorf("double-quoted variable = %q", w)
	}
}

func crushrcCrush(t *testing.T, content string) *Crush {
	t.Helper()
	c := &Crush{Home: t.TempDir(), Look: noBin}
	if content != "" {
		writeFile(t, c.crushrcFile(), content)
	}
	return c
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The provider block goes after everything the user wrote (later statements
// win) and clear gives the file back byte for byte.
func TestCrushProviderApplyClear(t *testing.T) {
	user := "#!/usr/bin/env bash\nmodel large anthropic/claude-sonnet-4\npermissions allow view\n"
	c := crushrcCrush(t, user)
	backups := t.TempDir()
	if err := c.ApplyProvider(ProviderProfile{Name: "proxy", BaseURL: "https://p.example/v1", Model: "m1", Token: "sk-'secret"}, backups); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, c.crushrcFile())
	if !strings.HasPrefix(got, user) || !strings.HasSuffix(got, "model large 'lazyagents/m1'\n# lazyagents — managed block end: provider\n") {
		t.Errorf("block not appended after the user's lines:\n%s", got)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(c.crushrcFile()); info.Mode().Perm() != 0o600 {
			t.Errorf("crushrc with a token has mode %v", info.Mode().Perm())
		}
	}
	p, ok, err := c.ReadProvider()
	if err != nil || !ok || p.Model != "m1" || p.BaseURL != "https://p.example/v1" || !p.HasToken || p.Token != "" {
		t.Fatalf("ReadProvider = %+v %v %v", p, ok, err)
	}
	// a second profile replaces the block; an env var is referenced, not copied
	if err := c.ApplyProvider(ProviderProfile{Name: "b", BaseURL: "http://l/v1", Model: "m2", EnvKey: "B_KEY", WireAPI: "anthropic"}, backups); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, c.crushrcFile()); strings.Count(got, "managed block start: provider") != 1 || !strings.Contains(got, `--api-key "$B_KEY"`) || strings.Contains(got, "secret") {
		t.Errorf("second apply:\n%s", got)
	}
	if p, _, _ := c.ReadProvider(); p.EnvKey != "B_KEY" || p.HasToken || p.WireAPI != "anthropic" || p.Model != "m2" {
		t.Errorf("ReadProvider after the second apply = %+v", p)
	}
	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, c.crushrcFile()); got != user {
		t.Errorf("clear did not give the file back:\n%q", got)
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 3 {
		t.Errorf("%d backups, want one per write", len(entries))
	}
}

func TestCrushProviderRejects(t *testing.T) {
	c := crushrcCrush(t, "")
	for _, p := range []ProviderProfile{
		{Name: "no-url", Model: "m"},
		{Name: "no-model", BaseURL: "http://x"},
		{Name: "responses", BaseURL: "http://x", Model: "m", WireAPI: "responses"},
		{Name: "bad-env", BaseURL: "http://x", Model: "m", EnvKey: "$(evil)"},
	} {
		if err := c.ApplyProvider(p, ""); err == nil {
			t.Errorf("%s: want an error", p.Name)
		}
	}
	if _, err := os.Stat(c.crushrcFile()); !os.IsNotExist(err) {
		t.Error("a rejected profile must not write crushrc")
	}
	// lazyagents created crushrc: clear removes it instead of leaving it empty
	if err := c.ApplyProvider(ProviderProfile{Name: "ok", BaseURL: "http://x", Model: "m"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.crushrcFile()); !os.IsNotExist(err) {
		t.Error("an emptied crushrc should be removed")
	}
}

// A block without its end leaves the file's structure unknown: refused.
func TestCrushBrokenBlockRefused(t *testing.T) {
	broken := "permissions allow view\n# lazyagents — managed block start: provider (do not edit by hand)\nprovider add lazyagents\n"
	c := crushrcCrush(t, broken)
	if err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "http://x", Model: "m"}, ""); err == nil {
		t.Fatal("applied over a block without an end")
	}
	if got := readFile(t, c.crushrcFile()); got != broken {
		t.Error("the file was changed")
	}
}

func TestCrushHooks(t *testing.T) {
	c := crushrcCrush(t, "permissions allow bash\n")
	writeFile(t, c.configFile(), `{"hooks":{"PreToolUse":[{"matcher":"^edit$","command":"./guard.sh","timeout":5}]}}`)
	h := Hook{Event: HookPreToolUse, Matcher: "^bash$", Command: "echo 'it''s' $HOME", Timeout: 10}
	for range 2 { // idempotent
		if err := c.AddHook(h, ""); err != nil {
			t.Fatal(err)
		}
	}
	hooks, err := c.ReadHooks()
	if err != nil || len(hooks) != 2 || !slices.ContainsFunc(hooks, h.Same) {
		t.Fatalf("ReadHooks = %+v, %v", hooks, err)
	}
	if !slices.ContainsFunc(hooks, func(x Hook) bool { return x.Command == "./guard.sh" && x.Timeout == 5 }) {
		t.Errorf("the crush.json hook is not listed: %+v", hooks)
	}
	if got := readFile(t, c.crushrcFile()); strings.Count(got, "hook add") != 1 || !strings.Contains(got, "--name lazyagents-") {
		t.Errorf("crushrc:\n%s", got)
	}
	if err := c.RemoveHook(h, ""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, c.crushrcFile()); got != "permissions allow bash\n" {
		t.Errorf("remove did not give the file back: %q", got)
	}
	if !slices.Equal(c.HookEvents(), []string{HookPreToolUse}) {
		t.Errorf("HookEvents = %v", c.HookEvents())
	}
}

// Crush (or another lazyagents) writing crushrc in the middle of an edit: the
// edit is redone on the new content, so both changes are kept, and only the
// first attempt backs the file up.
func TestCrushrcConcurrentWrite(t *testing.T) {
	c := crushrcCrush(t, "permissions allow view\n")
	backups := t.TempDir()
	once := true
	crushBeforeWrite = func() {
		if once {
			once = false
			writeFile(t, c.crushrcFile(), "permissions allow view\noption progress false\n")
		}
	}
	t.Cleanup(func() { crushBeforeWrite = func() {} })
	if err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "http://x", Model: "m"}, backups); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, c.crushrcFile()); !strings.Contains(got, "option progress false") || !strings.Contains(got, "provider add lazyagents") {
		t.Errorf("a change was lost:\n%s", got)
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 1 {
		t.Errorf("%d backups, want 1", len(entries))
	}
}

func TestCrushrcFileFollowsGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	c := &Crush{Home: t.TempDir(), GlobalConfig: dir}
	if c.crushrcFile() != filepath.Join(dir, "crushrc") || c.HooksFile() != c.ProviderFile() {
		t.Errorf("crushrc = %s", c.crushrcFile())
	}
}

// A value with a line break would come back cut from crushrc (read one line
// at a time): refused, file untouched.
func TestCrushRefusesMultiLineValues(t *testing.T) {
	c := crushrcCrush(t, "permissions allow bash\n")
	multi := Hook{Event: HookPreToolUse, Command: "if true; then\n  echo hi\nfi"}
	if err := c.AddHook(multi, ""); err == nil {
		t.Error("a multi-line hook command was accepted")
	}
	marker := "x\n# lazyagents — managed block end: provider"
	if err := c.ApplyProvider(ProviderProfile{Name: marker, BaseURL: "http://x", Model: "m"}, ""); err == nil {
		t.Error("a multi-line profile value was accepted")
	}
	if got := readFile(t, c.crushrcFile()); got != "permissions allow bash\n" {
		t.Errorf("crushrc changed: %q", got)
	}
}

// A hook that lives in crush.json counts as installed, and lazyagents never
// claims to remove it from a file it does not edit.
func TestCrushJSONHookIsNotRemovedSilently(t *testing.T) {
	c := crushrcCrush(t, "")
	h := Hook{Event: HookPreToolUse, Matcher: "^edit$", Command: "./guard.sh"}
	writeFile(t, c.configFile(), `{"hooks":{"PreToolUse":[{"matcher":"^edit$","command":"./guard.sh"}]}}`)
	if err := c.AddHook(h, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.crushrcFile()); !os.IsNotExist(err) {
		t.Error("a hook already in crush.json was added to crushrc too")
	}
	if err := c.RemoveHook(h, ""); err == nil || !strings.Contains(err.Error(), "crush.json") {
		t.Errorf("RemoveHook of a crush.json hook = %v, want an error naming crush.json", err)
	}
}

// A literal token that starts with $ stays a token; only "$VAR" is a variable.
func TestCrushTokenStartingWithDollar(t *testing.T) {
	c := crushrcCrush(t, "")
	if err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "http://x", Model: "m", Token: "$abc"}, ""); err != nil {
		t.Fatal(err)
	}
	if p, _, _ := c.ReadProvider(); !p.HasToken || p.EnvKey != "" {
		t.Errorf("ReadProvider = %+v, want a token and no env var", p)
	}
}
