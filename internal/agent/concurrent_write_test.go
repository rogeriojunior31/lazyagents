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
