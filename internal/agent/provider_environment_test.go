package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeEnvironmentPrecedenceIsValueFreeAndReadOnly(t *testing.T) {
	t.Setenv("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST", "")
	t.Setenv(claudeEnvBaseURL, "https://inherited-private.example/?key=inherited-secret")
	t.Setenv(claudeEnvModel, "inherited-model")
	c, path := writeClaudeSettings(t, `{"model":"top-level-model","env":{"ANTHROPIC_BASE_URL":"https://file-private.example","ANTHROPIC_MODEL":"file-model","ANTHROPIC_AUTH_TOKEN":"native-secret"}}`)
	before, _ := os.ReadFile(path)
	got, err := c.ProviderEnvironment()
	if err != nil || len(got) != 3 {
		t.Fatalf("diagnostics = %+v, %v", got, err)
	}
	if got[0].Variable != claudeEnvBaseURL || got[0].OtherSource != "current process environment" || !strings.Contains(got[0].Explanation, "replaces the inherited value") {
		t.Fatalf("wrong settings.env precedence: %+v", got[0])
	}
	if got[2].Code != "environment_model_override" || !strings.Contains(got[2].Source, "(env)") {
		t.Fatalf("model source = %+v", got[2])
	}
	data, _ := json.Marshal(got)
	for _, secret := range []string{"inherited-private", "inherited-secret", "file-private", "inherited-model", "file-model", "top-level-model", "native-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("diagnostics serialized a value: %s", data)
		}
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) || os.Getenv(claudeEnvModel) != "inherited-model" {
		t.Fatal("diagnostics changed native configuration or process environment")
	}
	t.Setenv(claudeEnvBaseURL, "https://file-private.example")
	t.Setenv(claudeEnvModel, "file-model")
	got, err = c.ProviderEnvironment()
	if err != nil || len(got) != 1 || got[0].Code != "environment_model_override" {
		t.Fatalf("equal sources should not conflict: %+v, %v", got, err)
	}
	t.Setenv("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST", "1")
	if got, err = c.ProviderEnvironment(); err != nil || len(got) != 0 {
		t.Fatalf("hosted policy must not be guessed: %+v, %v", got, err)
	}
}

func TestClaudeInheritedModelAndLiteralEnvironment(t *testing.T) {
	t.Setenv("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST", "")
	t.Setenv(claudeEnvModel, "shell-model")
	c, _ := writeClaudeSettings(t, `{"model":"native-model","env":{"ANTHROPIC_AUTH_TOKEN":"$MISSING_TOKEN"}}`)
	got, err := c.ProviderEnvironment()
	if err != nil || len(got) != 1 || got[0].Source != "current process environment" || got[0].Code != "environment_model_override" {
		t.Fatalf("inherited model/literal credential = %+v, %v", got, err)
	}
}

func TestCodexMissingSelectedCredentialEnvironment(t *testing.T) {
	c := NewCodex(t.TempDir())
	const key = "LAZYAGENTS_M262_CREDENTIAL"
	if err := c.ApplyProvider(ProviderProfile{Name: "test", BaseURL: "https://example.test", EnvKey: key}, ""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "unrelated-secret")
	for _, value := range []string{"", " \t", "present-secret"} {
		t.Setenv(key, value)
		got, err := c.ProviderEnvironment()
		if err != nil {
			t.Fatal(err)
		}
		if value == "present-secret" {
			if len(got) != 0 {
				t.Fatalf("present credential reported missing: %+v", got)
			}
		} else if len(got) != 1 || got[0].Variable != key || !strings.Contains(got[0].Source, "env_key") {
			t.Fatalf("missing selected reference = %+v", got)
		}
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	if got, err := c.ProviderEnvironment(); err != nil || len(got) != 1 || got[0].Variable != key {
		t.Fatalf("unset reference = %+v, %v", got, err)
	}
	writeEnvironmentFixture(t, c.ProviderFile(), "model_provider = \"custom\"\n[model_providers.custom]\nenv_key = \"not-a-name-private-sentinel\"\n")
	got, err := c.ProviderEnvironment()
	data, _ := json.Marshal(got)
	if err == nil || len(got) != 0 || strings.Contains(err.Error()+string(data), "private-sentinel") {
		t.Fatalf("unsafe invalid reference = %s, %v", data, err)
	}
	if p, active, err := c.ReadProvider(); err == nil || active || p != (ProviderProfile{}) {
		t.Fatalf("invalid reference entered a public profile: %+v, %v", p, err)
	}
	// Unselected references, built-in login and keyless endpoints are not requirements.
	writeEnvironmentFixture(t, c.ProviderFile(), "model_provider = \"openai\"\n[model_providers.unselected]\nenv_key = \""+key+"\"\n")
	t.Setenv(key, "")
	if got, err := c.ProviderEnvironment(); err != nil || len(got) != 0 {
		t.Fatalf("unselected reference inspected: %+v, %v", got, err)
	}
}

func TestPiCredentialPriorityAndExplicitReferences(t *testing.T) {
	p := NewPi(t.TempDir())
	const key = "LAZYAGENTS_M262_PI_KEY"
	t.Setenv(key, "")
	if err := p.ApplyProvider(ProviderProfile{Name: "test", BaseURL: "https://example.test", Model: "m", EnvKey: key}, ""); err != nil {
		t.Fatal(err)
	}
	if got, err := p.ProviderEnvironment(); err != nil || len(got) != 1 || got[0].Variable != key {
		t.Fatalf("Pi missing reference = %+v, %v", got, err)
	}
	for _, kind := range []string{"api_key", "oauth"} {
		writeEnvironmentFixture(t, filepath.Join(p.agentDir(), "auth.json"), `{"lazyagents":{"type":"`+kind+`","key":"stored-secret","access":"oauth-secret"}}`)
		if got, err := p.ProviderEnvironment(); err != nil || len(got) != 0 {
			t.Fatalf("stored credential has priority: %+v, %v", got, err)
		}
	}
	if err := os.Remove(filepath.Join(p.agentDir(), "auth.json")); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"literal-secret", "$$escaped-secret", "$!literal-secret", "!touch should-never-run", "${MALFORMED-secret}", "${" + key + "}", "$" + key} {
		data, _ := json.Marshal(map[string]any{"providers": map[string]any{"lazyagents": map[string]any{"baseUrl": "https://example.test", "apiKey": value}}})
		writeEnvironmentFixture(t, p.ProviderFile(), string(data))
		got, err := p.ProviderEnvironment()
		isReference := value == "$"+key || value == "${"+key+"}"
		if err != nil || (len(got) == 1) != isReference {
			t.Fatalf("reference %q = %+v, %v", value, got, err)
		}
		profile, _, _ := p.ReadProvider()
		if !isReference && profile.EnvKey != "" {
			t.Fatalf("literal exposed as environment name: %+v", profile)
		}
	}
}

func TestCrushManagedReferenceDoesNotExecuteScripts(t *testing.T) {
	c := NewCrush(t.TempDir())
	const key = "LAZYAGENTS_M262_CRUSH_KEY"
	t.Setenv(key, "")
	if err := c.ApplyProvider(ProviderProfile{Name: "test", BaseURL: "https://example.test", Model: "m", EnvKey: key}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	before := "touch " + shQuote(marker) + "\nexport " + key + "=from-script\n" + string(data)
	writeEnvironmentFixture(t, c.ProviderFile(), before)
	got, err := c.ProviderEnvironment()
	if err != nil || len(got) != 1 || got[0].Variable != key || !strings.Contains(got[0].Explanation, "effective environment is unknown") {
		t.Fatalf("Crush script scope = %+v, %v", got, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("native script executed: %v", err)
	}
	after, _ := os.ReadFile(c.ProviderFile())
	if string(after) != before {
		t.Fatal("crushrc changed")
	}
	t.Setenv(key, "present-secret")
	if got, err := c.ProviderEnvironment(); err != nil || len(got) != 0 {
		t.Fatalf("present Crush reference = %+v, %v", got, err)
	}
}

func writeEnvironmentFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
