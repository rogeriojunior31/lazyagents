package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderValidationIsReadOnlyAndChecksNativeFiles(t *testing.T) {
	for _, id := range []string{"claude", "codex", "pi", "crush"} {
		t.Run(id, func(t *testing.T) {
			home := t.TempDir()
			var host ProviderHost
			switch id {
			case "claude":
				host = NewClaude(home)
			case "codex":
				host = NewCodex(home)
			case "pi":
				host = NewPi(home)
			case "crush":
				host = NewCrush(home)
			}
			p := ProviderProfile{Name: "local", BaseURL: "http://localhost:11434/v1", Model: "m", EnvKey: "UNVERIFIED_TEST_KEY"}
			v, err := host.(ProviderValidator).ValidateProvider(p)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(v)
			if !strings.Contains(string(data), "not been authenticated") || !strings.Contains(string(data), "new agent session") {
				t.Fatalf("requirements missing: %s", data)
			}
			files, _ := os.ReadDir(home)
			if len(files) != 0 {
				t.Fatal("validation created files")
			}
			for _, pr := range []ProviderProfile{
				{BaseURL: "ftp://example.test", Model: "m"},
				{BaseURL: "https://user:secret@example.test", Model: "m"},
				{BaseURL: "https://example.test", Model: "m", EnvKey: "NOT-A-NAME"},
			} {
				if _, err := host.(ProviderValidator).ValidateProvider(pr); err == nil {
					t.Fatalf("invalid fields accepted: %+v", pr.Redacted())
				}
				if err := host.ApplyProvider(pr, filepath.Join(home, "backups")); err == nil {
					t.Fatal("direct apply bypassed validation")
				}
			}
			path, malformed := host.ProviderFile(), "invalid config"
			switch id {
			case "claude":
				malformed = `{"env": []}`
			case "codex":
				malformed = codexBlockStart + "\nmodel = \"m\"\n"
			case "pi":
				path = host.(*Pi).settingsFile() // must check Pi's second file as well
				malformed = `[]`
			case "crush":
				malformed = "# lazyagents — managed block start: provider (do not edit by hand)\n"
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(malformed), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := host.(ProviderValidator).ValidateProvider(p); err == nil {
				t.Fatal("malformed native file accepted")
			}
			after, _ := os.ReadFile(path)
			if string(after) != malformed {
				t.Fatal("validation rewrote native file")
			}
		})
	}
}

func TestCodexValidationUsesRedactedTokenPresence(t *testing.T) {
	c := NewCodex(t.TempDir())
	p := ProviderProfile{BaseURL: "https://example.test", HasToken: true}
	if _, err := c.ValidateProvider(p); err == nil || !strings.Contains(err.Error(), "envKey") {
		t.Fatalf("missing envKey: %v", err)
	}
	p.EnvKey = "UNVERIFIED_TEST_KEY"
	t.Setenv(p.EnvKey, "")
	if _, err := c.ValidateProvider(p); err != nil {
		t.Fatalf("a syntactically valid reference should not assert authentication: %v", err)
	}
	p.WireAPI = "chat"
	if _, err := c.ValidateProvider(p); err == nil {
		t.Fatal("Codex accepted Chat Completions")
	}
}
