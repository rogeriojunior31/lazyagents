package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// A JSON config the agent wrote between our read and our write is never
// overwritten: the save fails and the other write survives.
func TestSettingsSaveRefusesConcurrentWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, path, `{"a":1}`)
	s, err := readSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{"a":1,"fromAgent":true}`)
	if err := s.set("mine", true); err != nil {
		t.Fatal(err)
	}
	if err := s.save(""); !errors.Is(err, fsutil.ErrChanged) {
		t.Fatalf("save = %v, want ErrChanged", err)
	}
	if data, _ := os.ReadFile(path); string(data) != `{"a":1,"fromAgent":true}` {
		t.Errorf("the agent's write was lost: %s", data)
	}
}

// Codex writing config.toml while lazyagents applies a provider: the edit is
// redone on the new file, so both changes end up in it.
func TestCodexTOMLRetriesConcurrentWrite(t *testing.T) {
	c := NewCodex(t.TempDir())
	writeFile(t, c.ProviderFile(), "model = \"gpt-5\"\n")
	once := true
	beforeTOMLWrite = func() {
		if once {
			once = false
			writeFile(t, c.ProviderFile(), "model = \"gpt-5\"\n\n[projects.\"/home/me/app\"]\ntrust_level = \"trusted\"\n")
		}
	}
	t.Cleanup(func() { beforeTOMLWrite = func() {} })
	if err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "http://localhost/v1", EnvKey: "K"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if !strings.Contains(string(data), `[projects."/home/me/app"]`) || !strings.Contains(string(data), `model_provider = "lazyagents"`) {
		t.Errorf("a change was lost:\n%s", data)
	}
}

// Pi rewriting settings.json in the middle of an apply: the next attempt
// completes it, the user's latest choice is the one clear gives back, and
// only the first attempt backs files up.
func TestPiProviderRetriesConcurrentWrite(t *testing.T) {
	p := &Pi{Home: t.TempDir(), Look: noBin}
	backups := t.TempDir()
	writeFile(t, p.settingsFile(), `{"defaultProvider":"openrouter","defaultModel":"a"}`)
	writeFile(t, p.ProviderFile(), `{"providers":{"ollama":{"baseUrl":"http://x"}}}`)
	once := true
	piBeforeSettingsSave = func() {
		if once {
			once = false
			writeFile(t, p.settingsFile(), `{"defaultProvider":"openrouter","defaultModel":"b"}`)
		}
	}
	t.Cleanup(func() { piBeforeSettingsSave = func() {} })
	if err := p.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "http://l/v1", Model: "m"}, backups); err != nil {
		t.Fatal(err)
	}
	if got, ok, _ := p.ReadProvider(); !ok || got.Model != "m" {
		t.Fatalf("apply not completed: %+v %v", got, ok)
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 2 {
		t.Errorf("%d backups, want 2 (one per file, first attempt only)", len(entries))
	}
	if err := p.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p.settingsFile())
	if !strings.Contains(string(data), `"defaultModel": "b"`) {
		t.Errorf("clear did not give back the user's latest model:\n%s", data)
	}
}

// Codex retries do not add backups of short-lived states.
func TestCodexTOMLRetryBacksUpOnce(t *testing.T) {
	c := NewCodex(t.TempDir())
	backups := t.TempDir()
	writeFile(t, c.ProviderFile(), "model = \"gpt-5\"\n")
	once := true
	beforeTOMLWrite = func() {
		if once {
			once = false
			writeFile(t, c.ProviderFile(), "model = \"gpt-5\"\n# edited by Codex\n")
		}
	}
	t.Cleanup(func() { beforeTOMLWrite = func() {} })
	if err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "http://l/v1", EnvKey: "K"}, backups); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 1 {
		t.Errorf("%d backups, want 1", len(entries))
	}
}

// Removing a file lazyagents created is refused when another program wrote it.
func TestSettingsRemoveRefusesConcurrentWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, path, `{}`)
	s, err := readSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{"permissions":{}}`)
	if err := s.remove(""); !errors.Is(err, fsutil.ErrChanged) {
		t.Fatalf("remove = %v, want ErrChanged", err)
	}
	if data, _ := os.ReadFile(path); string(data) != `{"permissions":{}}` {
		t.Errorf("the other write was lost: %q", data)
	}
}
