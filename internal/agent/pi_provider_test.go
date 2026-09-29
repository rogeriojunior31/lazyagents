package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return m
}

func TestPiProviderApplyClear(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".pi", "agent")
	p := &Pi{Home: home, Look: noBin}
	backups := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.json"), `{"theme":"dark","defaultProvider":"openrouter","defaultModel":"x/y"}`)
	writeFile(t, filepath.Join(dir, "models.json"), `{"providers":{"ollama":{"baseUrl":"http://localhost:11434/v1","api":"openai-completions","models":[{"id":"qwen"}]}}}`)

	prof := ProviderProfile{Name: "proxy", BaseURL: "https://proxy.example/v1", Model: "gpt-x", Token: "sk-secret"}
	if err := p.ApplyProvider(prof, backups); err != nil {
		t.Fatal(err)
	}
	models := readJSON(t, p.ProviderFile())
	provs := models["providers"].(map[string]any)
	ours := provs["lazyagents"].(map[string]any)
	if ours["baseUrl"] != prof.BaseURL || ours["api"] != "openai-completions" || ours["apiKey"] != "sk-secret" || provs["ollama"] == nil {
		t.Fatalf("models.json = %v", models)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(p.ProviderFile()); info.Mode().Perm() != 0o600 {
			t.Errorf("models.json mode = %v, want 0600", info.Mode().Perm())
		}
	}
	settings := readJSON(t, p.settingsFile())
	if settings["defaultProvider"] != "lazyagents" || settings["defaultModel"] != "gpt-x" || settings["theme"] != "dark" {
		t.Fatalf("settings.json = %v", settings)
	}
	got, ok, err := p.ReadProvider()
	if err != nil || !ok || got.Token != "" || !got.HasToken || got.Model != "gpt-x" || got.BaseURL != prof.BaseURL {
		t.Fatalf("ReadProvider = %+v %v %v", got, ok, err)
	}
	if entries, _ := os.ReadDir(backups); len(entries) < 2 {
		t.Errorf("backups = %d, want both files backed up", len(entries))
	}

	// a second profile replaces the first but keeps the user's original defaults
	if err := p.ApplyProvider(ProviderProfile{Name: "b", BaseURL: "https://b/v1", Model: "m2", EnvKey: "B_KEY", WireAPI: "responses"}, backups); err != nil {
		t.Fatal(err)
	}
	got, _, _ = p.ReadProvider()
	if got.EnvKey != "B_KEY" || got.HasToken || got.WireAPI != "responses" || got.Model != "m2" {
		t.Errorf("ReadProvider after the second apply = %+v", got)
	}
	if key := readJSON(t, p.ProviderFile())["providers"].(map[string]any)["lazyagents"].(map[string]any)["apiKey"]; key != "$B_KEY" {
		t.Errorf("apiKey = %v, want $B_KEY", key)
	}

	if err := p.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	settings = readJSON(t, p.settingsFile())
	if settings["defaultProvider"] != "openrouter" || settings["defaultModel"] != "x/y" || settings["theme"] != "dark" {
		t.Errorf("settings.json after clear = %v", settings)
	}
	provs = readJSON(t, p.ProviderFile())["providers"].(map[string]any)
	if provs["lazyagents"] != nil || provs["ollama"] == nil {
		t.Errorf("providers after clear = %v", provs)
	}
	if _, ok, _ := p.ReadProvider(); ok {
		t.Error("ReadProvider after clear should be empty")
	}
}

// With no models.json and no defaults before, clearing leaves nothing behind.
func TestPiProviderClearRemovesWhatItCreated(t *testing.T) {
	home := t.TempDir()
	p := &Pi{Home: home, Look: noBin}
	writeFile(t, p.settingsFile(), `{"theme":"light"}`)
	if err := p.ApplyProvider(ProviderProfile{Name: "a", BaseURL: "http://x/v1", Model: "m"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProviderFile()); !os.IsNotExist(err) {
		t.Error("models.json created by the profile should be gone")
	}
	settings := readJSON(t, p.settingsFile())
	if len(settings) != 1 || settings["theme"] != "light" {
		t.Errorf("settings.json = %v", settings)
	}
}

// When the user picks another provider in pi, the profile no longer counts as
// applied and clearing does not take the user's choice away.
func TestPiProviderUserSwitchedAway(t *testing.T) {
	home := t.TempDir()
	p := &Pi{Home: home, Look: noBin}
	if err := p.ApplyProvider(ProviderProfile{Name: "a", BaseURL: "http://x/v1", Model: "m"}, ""); err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.settingsFile(), `{"defaultProvider":"anthropic","defaultModel":"claude-x"}`)
	if _, ok, _ := p.ReadProvider(); ok {
		t.Error("a profile the user switched away from is not applied")
	}
	if err := p.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	if s := readJSON(t, p.settingsFile()); s["defaultProvider"] != "anthropic" || s["defaultModel"] != "claude-x" {
		t.Errorf("the user's choice was changed: %v", s)
	}
}

func TestPiProviderRejects(t *testing.T) {
	p := &Pi{Home: t.TempDir(), Look: noBin}
	for _, pr := range []ProviderProfile{
		{Name: "no-model", BaseURL: "http://x"},
		{Name: "no-url", Model: "m"},
		{Name: "bad-wire", BaseURL: "http://x", Model: "m", WireAPI: "grpc"},
	} {
		if err := p.ApplyProvider(pr, ""); err == nil {
			t.Errorf("%s: want an error", pr.Name)
		}
	}
	if _, err := os.Stat(p.ProviderFile()); !os.IsNotExist(err) {
		t.Error("a rejected profile must not write models.json")
	}
	if err := p.ApplyProvider(ProviderProfile{Name: "ok", BaseURL: "http://x", Model: "m", WireAPI: "anthropic"}, ""); err != nil {
		t.Fatal(err)
	}
	if api := readJSON(t, p.ProviderFile())["providers"].(map[string]any)["lazyagents"].(map[string]any)["api"]; !strings.HasPrefix(api.(string), "anthropic") {
		t.Errorf("api = %v", api)
	}
}

// Pi hides a custom provider without a credential, so a keyless profile gets a
// dummy key; it does not count as a saved token.
func TestPiProviderKeyless(t *testing.T) {
	p := &Pi{Home: t.TempDir(), Look: noBin}
	if err := p.ApplyProvider(ProviderProfile{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "qwen"}, ""); err != nil {
		t.Fatal(err)
	}
	if key := readJSON(t, p.ProviderFile())["providers"].(map[string]any)["lazyagents"].(map[string]any)["apiKey"]; key != piNoKey {
		t.Errorf("apiKey = %v, want the dummy key", key)
	}
	if got, ok, _ := p.ReadProvider(); !ok || got.HasToken || got.EnvKey != "" {
		t.Errorf("ReadProvider = %+v %v", got, ok)
	}
}

// Switching away in pi and applying again records the user's new choice, and
// a lost state file never makes "lazyagents" the provider to go back to.
func TestPiProviderRecordsCurrentDefault(t *testing.T) {
	home := t.TempDir()
	p := &Pi{Home: home, Look: noBin}
	prof := ProviderProfile{Name: "a", BaseURL: "http://x/v1", Model: "m"}
	writeFile(t, p.settingsFile(), `{"defaultProvider":"openrouter","defaultModel":"r1"}`)
	if err := p.ApplyProvider(prof, ""); err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.settingsFile(), `{"defaultProvider":"openai","defaultModel":"gpt"}`)
	if err := p.ApplyProvider(prof, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	if s := readJSON(t, p.settingsFile()); s["defaultProvider"] != "openai" || s["defaultModel"] != "gpt" {
		t.Errorf("after switching away: %v", s)
	}

	// state lost while ours is the default: clear drops the keys (pi's default)
	if err := p.ApplyProvider(prof, ""); err != nil {
		t.Fatal(err)
	}
	p.providerMem = nil
	if err := p.ApplyProvider(prof, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	if s := readJSON(t, p.settingsFile()); s["defaultProvider"] != nil || s["defaultModel"] != nil {
		t.Errorf("after a lost state: %v", s)
	}
}
