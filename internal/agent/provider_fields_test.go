package agent

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestProviderFieldsReflectNativeDefaultsAndCredentialPrecedence(t *testing.T) {
	p := ProviderProfile{Name: "ignored-name", BaseURL: "https://example.test/v1", Model: "m", Token: "never-export-fixture-secret", EnvKey: "KEY_A"}
	readers := []ProviderFieldReader{NewClaude(t.TempDir()), NewCodex(t.TempDir()), NewPi(t.TempDir()), NewCrush(t.TempDir())}
	protocols := []string{"anthropic", "responses", "chat", "chat"}
	for i, reader := range readers {
		fields := reader.ProviderFields(p)
		if fields["protocol"] != protocols[i] || fields["endpoint"] != p.BaseURL || fields["model"] != p.Model {
			t.Fatalf("wrong native fields: %v", fields)
		}
		data, _ := json.Marshal(fields)
		if strings.Contains(string(data), p.Token) || strings.Contains(string(data), p.Name) {
			t.Fatal("secret/name escaped public projection")
		}
		changed := p
		changed.Token = "another-fixture-secret"
		if !reflect.DeepEqual(fields, reader.ProviderFields(changed)) {
			t.Fatal("literal credentials were compared")
		}
		changed.EnvKey = "KEY_B"
		same := reflect.DeepEqual(fields, reader.ProviderFields(changed))
		if same != (i != 1) {
			t.Fatal("credential precedence did not match native writes")
		}
	}
	claude := NewClaude(t.TempDir())
	p.WireAPI = "responses"
	if claude.ProviderFields(p)["protocol"] != "anthropic" {
		t.Fatal("Claude protocol selector was treated as effective")
	}
	p.Model, p.BaseURL, p.Token = "", "", ""
	fields := claude.ProviderFields(p)
	for _, key := range []string{"model", "endpoint", "credential"} {
		if _, ok := fields[key]; ok {
			t.Fatal("unset native fields were constrained")
		}
	}
	pi := NewPi(t.TempDir())
	for alias, want := range map[string]string{"": "chat", "openai-completions": "chat", "openai-responses": "responses", "anthropic-messages": "anthropic", "unknown-native-api": "unknown-native-api"} {
		fields := pi.ProviderFields(ProviderProfile{WireAPI: alias})
		if fields["protocol"] != want {
			t.Fatal("native protocol alias lost")
		}
	}
}
