package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

func openCodeFixture(t *testing.T) *OpenCode {
	t.Helper()
	o := NewOpenCode(t.TempDir())
	o.ProviderState = filepath.Join(t.TempDir(), "opencode-provider-state.json")
	data, err := os.ReadFile(filepath.Join(fixturesDir, "opencode", "2.0.32", "home", ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, o.ProviderFile(), string(data))
	return o
}

func TestRecordedOpenCodeProviderRoundTrip(t *testing.T) {
	var proof struct {
		Version, OS, Arch, Recorded, How string
		Before                           json.RawMessage
		Probes                           []struct {
			Credential                 string
			Applied, Cleared           json.RawMessage
			AppliedModel, ClearedModel string
		}
	}
	data, err := os.ReadFile(filepath.Join(fixturesDir, "opencode", "2.0.32", "provider-roundtrip.json"))
	if err != nil || json.Unmarshal(data, &proof) != nil {
		t.Fatalf("invalid recorded provider evidence: %v", err)
	}
	if proof.Version != "2.0.32" || proof.OS != "linux" || proof.Arch != "amd64" || proof.Recorded == "" || proof.How == "" || len(proof.Probes) != 3 {
		t.Fatal("incomplete native recording origin")
	}
	for _, probe := range proof.Probes {
		t.Run(probe.Credential, func(t *testing.T) {
			o := NewOpenCode(t.TempDir())
			writeFile(t, o.ProviderFile(), string(proof.Before))
			p := ProviderProfile{Name: "fixture-managed", BaseURL: "http://127.0.0.1:18765/v1", Model: "managed-fixture-model-" + probe.Credential, WireAPI: "chat"}
			switch probe.Credential {
			case "token":
				p.Token = "fixture-managed-key"
			case "environment":
				p.EnvKey = "FIXTURE_OPEN_CODE_KEY"
			case "keyless":
			default:
				t.Fatal("unknown native credential probe")
			}
			if probe.AppliedModel != p.Model || probe.ClearedModel != "fake-model" {
				t.Fatal("native inference used the wrong model")
			}
			assertJSON := func(want json.RawMessage) {
				t.Helper()
				actual, _ := os.ReadFile(o.ProviderFile())
				a, e := recoveryJSONFields(actual)
				b, f := recoveryJSONFields(want)
				if e != nil || f != nil || !reflect.DeepEqual(a, b) {
					t.Fatalf("adapter no longer matches CLI-verified config:\n%s", actual)
				}
			}
			if err := o.ApplyProvider(p, t.TempDir()); err != nil {
				t.Fatal(err)
			}
			assertJSON(probe.Applied)
			if err := o.ClearProvider(t.TempDir()); err != nil {
				t.Fatal(err)
			}
			assertJSON(probe.Cleared)
			assertJSON(proof.Before)
		})
	}
}

func TestOpenCodeProviderRoundTripAndOwnership(t *testing.T) {
	o := openCodeFixture(t)
	before := fixtureConfigs(t, o.Home)
	p := ProviderProfile{Name: "fixture", BaseURL: "http://127.0.0.1:18765/v1", Model: "new/model", Token: "synthetic-private-token", EnvKey: "IGNORED_KEY"}
	backups := t.TempDir()
	preview, err := o.PreviewProvider(&p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(o.ProviderState); !os.IsNotExist(err) {
		t.Fatal("preview wrote ownership")
	}
	if err := o.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	got, active, err := o.ReadProvider()
	if err != nil || !active || got.Model != p.Model || got.Token != "" || !got.HasToken || got.EnvKey != "" {
		t.Fatalf("provider = %+v, %v, %v", got, active, err)
	}
	native := readFile(t, o.ProviderFile())
	if !strings.Contains(native, `"fake"`) || !strings.Contains(native, `"autoupdate": false`) {
		t.Fatal("foreign provider/settings lost during apply")
	}
	state := readFile(t, o.ProviderState)
	if strings.Contains(state, p.Token) || strings.Contains(state, p.BaseURL) || strings.Contains(state, `"apiKey"`) {
		t.Fatal("secret/config payload in ownership metadata")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(o.ProviderFile())
		meta, _ := os.Stat(o.ProviderState)
		if info.Mode().Perm() != 0o600 || meta.Mode().Perm() != 0o600 {
			t.Fatal("private file permissions missing")
		}
	}
	stable, err := o.PreviewProvider(&p)
	if err != nil || stable.Files[0].Action != "unchanged" || len(stable.Files[0].Changes) != 0 || stable.Revision == preview.Revision {
		t.Fatalf("unstable preview: %+v %v", stable, err)
	}
	p.Model = "other"
	if err := o.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	// Recreate the adapter: restoration must survive restarting lazyagents.
	next := NewOpenCode(o.Home)
	next.ProviderState = o.ProviderState
	clear, err := next.PreviewProvider(nil)
	if err != nil || !clear.Managed {
		t.Fatalf("clear: %+v, %v", clear, err)
	}
	if err := next.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	if after := fixtureConfigs(t, o.Home); !reflect.DeepEqual(canonicalFixtureConfigs(before), canonicalFixtureConfigs(after)) {
		t.Fatalf("foreign settings changed: %v", after)
	}
	for _, file := range snapshotFiles(t, backups) {
		var record fsutil.BackupRecord
		data, err := os.ReadFile(fsutil.BackupMetadataPath(filepath.Join(backups, file.Name())))
		if err != nil || json.Unmarshal(data, &record) != nil || record.Agent != o.ID() || !strings.HasPrefix(record.Operation, "provider.") {
			t.Fatalf("missing backup provenance: %v", err)
		}
	}
}

func TestOpenCodeProviderRefusesUnsafeNativeConfig(t *testing.T) {
	for _, input := range []string{
		`{"provider":{"lazyagents":{"options":{"apiKey":"native-private"}}}}`,
		`{"providers":{"lazyagents":{}}}`,
		`{"provider":[]}`, `{"providers":null}`, `{"model":42}`,
		`{"model":"one/x","model":"two/y"}`, `{"provider":{"one":{},"one":{}}}`,
		`{"model":"{file:native-private}"}`, `{"model":{"providerID":"fake","model":"m","secret":"native-private"}}`,
		`{"disabled_providers":["lazyagents"]}`, `{"enabled_providers":["fake"]}`,
		`{"provider":{}} trailing-native-private`, `{"provider":{},}`, `{"provider":{/*native-private*/}}`,
	} {
		t.Run(input, func(t *testing.T) {
			o := NewOpenCode(t.TempDir())
			o.ProviderState = filepath.Join(t.TempDir(), "state.json")
			writeFile(t, o.ProviderFile(), input)
			p := ProviderProfile{Name: "p", BaseURL: "http://localhost/v1", Model: "m"}
			_, err := o.ValidateProvider(p)
			if err == nil || strings.Contains(err.Error(), "native-private") {
				t.Fatalf("unsafe validation: %v", err)
			}
			if err := o.ApplyProvider(p, t.TempDir()); err == nil {
				t.Fatal("direct apply bypassed preflight")
			}
			if readFile(t, o.ProviderFile()) != input {
				t.Fatal("native file modified")
			}
			if _, err := os.Stat(o.ProviderState); !os.IsNotExist(err) {
				t.Fatal("rejection wrote ownership")
			}
		})
	}
}

func TestOpenCodeProviderExternalEditsAndConditionalWrites(t *testing.T) {
	for _, editEntry := range []bool{false, true} {
		o := openCodeFixture(t)
		p := ProviderProfile{Name: "p", BaseURL: "http://localhost/v1", Model: "m"}
		if err := o.ApplyProvider(p, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		s, _ := readSettings(o.ProviderFile())
		if editEntry {
			providers, _, _ := openCodeProviders(s, "provider")
			_ = providers.set("lazyagents", map[string]string{"name": "external"})
			_ = s.set("provider", providers)
		} else {
			_ = s.set("model", "fake/user-choice")
		}
		if err := s.save(""); err != nil {
			t.Fatal(err)
		}
		before := readFile(t, o.ProviderFile())
		err := o.ClearProvider(t.TempDir())
		if editEntry {
			if err == nil || before != readFile(t, o.ProviderFile()) {
				t.Fatal("clear overwrote edited provider")
			}
		} else if err != nil || !strings.Contains(readFile(t, o.ProviderFile()), "fake/user-choice") || strings.Contains(readFile(t, o.ProviderFile()), `"lazyagents"`) {
			t.Fatalf("clear lost user's model: %v", err)
		}
	}
	o := openCodeFixture(t)
	p := ProviderProfile{Name: "p", BaseURL: "http://localhost/v1", Model: "m"}
	beforeOpenCodeProviderWrite = func() { writeFile(t, o.ProviderFile(), `{"model":"external/new"}`) }
	t.Cleanup(func() { beforeOpenCodeProviderWrite = func() {} })
	if err := o.ApplyProvider(p, t.TempDir()); !errors.Is(err, fsutil.ErrChanged) {
		t.Fatalf("concurrent native change overwritten: %v", err)
	}
	beforeOpenCodeProviderWrite = func() {}
	if readFile(t, o.ProviderFile()) != `{"model":"external/new"}` {
		t.Fatal("concurrent data lost")
	}
	if err := o.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := o.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, o.ProviderFile()), "external/new") {
		t.Fatal("retry restored stale original model")
	}
}

func TestOpenCodeProviderCredentialsAndNativeV2Selection(t *testing.T) {
	for _, name := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT", "OPENCODE_CONFIG_DIR"} {
		t.Setenv(name, "")
	}
	for _, key := range []string{"", "{env:FIXTURE_OPEN_CODE_KEY}", "native-private", "{file:/never/read/credential}"} {
		o := NewOpenCode(t.TempDir())
		raw, _ := json.Marshal(map[string]any{"model": map[string]string{"providerID": "foreign", "model": "m"}, "providers": map[string]any{"foreign": map[string]any{"package": "@opencode/ai/providers/openai-compatible", "settings": map[string]string{"baseURL": "http://localhost/v1", "apiKey": key}, "models": map[string]any{"m": map[string]string{"name": "M"}}}}})
		writeFile(t, o.ProviderFile(), string(raw))
		p, active, err := o.ReadProvider()
		if err != nil || !active || p.Token != "" || p.Model != "m" || p.HasToken != (key != "" && !strings.HasPrefix(key, "{env:")) {
			t.Fatalf("unsafe native read: %+v, %v", p, err)
		}
		if key == "{env:FIXTURE_OPEN_CODE_KEY}" {
			t.Setenv("FIXTURE_OPEN_CODE_KEY", "")
			diagnostics, err := o.ProviderEnvironment()
			if err != nil || len(diagnostics) != 1 || diagnostics[0].Variable != "FIXTURE_OPEN_CODE_KEY" {
				t.Fatalf("missing environment diagnostic: %+v, %v", diagnostics, err)
			}
		}
		before := fixtureConfigs(t, o.Home)
		pr := ProviderProfile{Name: "managed", BaseURL: "http://localhost/v1", Model: "m", EnvKey: "FIXTURE_OPEN_CODE_KEY"}
		if err := o.ApplyProvider(pr, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if err := o.ClearProvider(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(canonicalFixtureConfigs(before), canonicalFixtureConfigs(fixtureConfigs(t, o.Home))) {
			t.Fatal("native V2 configuration changed")
		}
	}
}

func TestOpenCodeProviderKeepsAdvancedEditsAndNativeLogin(t *testing.T) {
	o := openCodeFixture(t)
	login := filepath.Join(o.Home, ".local", "share", "opencode", "auth.json")
	writeFile(t, login, "native-login-canary: intentionally not JSON")
	p := ProviderProfile{Name: "p", BaseURL: "http://localhost/v1", Model: "m"}
	if err := o.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	s, _ := readSettings(o.ProviderFile())
	_ = s.set("model", "lazyagents/user-selected-other-model")
	if err := s.save(""); err != nil {
		t.Fatal(err)
	}
	if err := o.ApplyProvider(p, t.TempDir()); err == nil {
		t.Fatal("reapply would restore a dangling managed model")
	}
	_ = s.set("model", "lazyagents/m")
	providers, _, _ := openCodeProviders(s, "provider")
	entry, _ := providers.raw("lazyagents")
	obj, _ := openCodeObject(entry)
	_ = obj.set("headers", map[string]string{"X-Foreign": "keep"})
	_ = providers.set("lazyagents", obj)
	_ = s.set("provider", providers)
	if err := s.save(""); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, o.ProviderFile())
	if err := o.ApplyProvider(p, t.TempDir()); err == nil {
		t.Fatal("reapply discarded advanced user content")
	}
	if err := o.ClearProvider(t.TempDir()); err == nil {
		t.Fatal("clear discarded advanced user content")
	}
	if readFile(t, o.ProviderFile()) != before || readFile(t, login) != "native-login-canary: intentionally not JSON" {
		t.Fatal("foreign config or login was changed")
	}
}

func TestOpenCodeProviderDoesNotProjectUnsupportedModelOverrides(t *testing.T) {
	o := NewOpenCode(t.TempDir())
	writeFile(t, o.ProviderFile(), `{"model":"foreign/m","provider":{"foreign":{"npm":"@ai-sdk/openai-compatible","options":{"baseURL":"https://parent.test"},"models":{"m":{"options":{"baseURL":"https://different.test"}}}}}}`)
	p, active, err := o.ReadProvider()
	if err != nil || !active || p.BaseURL != "" || p.WireAPI != "native" {
		t.Fatalf("misleading parent projection: %+v, %v", p, err)
	}
	if err := o.ApplyProvider(ProviderProfile{Name: "p", BaseURL: "http://localhost/v1", Model: "m"}, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCodeProviderReadKeepsPrivateAndUnknownFieldsHidden(t *testing.T) {
	for _, endpoint := range []string{"https://user:native-private@example.test/v1", "{file:/native-private}"} {
		o := NewOpenCode(t.TempDir())
		data, _ := json.Marshal(map[string]any{"model": "fake/m", "provider": map[string]any{"fake": map[string]any{"npm": "@ai-sdk/openai-compatible", "options": map[string]string{"baseURL": endpoint, "apiKey": "native-private"}}}})
		writeFile(t, o.ProviderFile(), string(data))
		p, _, err := o.ReadProvider()
		if err == nil || strings.Contains(err.Error(), "native-private") || p.BaseURL != "" || p.Token != "" {
			t.Fatalf("unsafe read: %+v, %v", p, err)
		}
	}
	o := NewOpenCode(t.TempDir())
	writeFile(t, o.ProviderFile(), `{"model":"fake/m","provider":{"fake":{"npm":"@ai-sdk/openai-compatible","options":{"baseurl":"https://ignored.test","APIKEY":"native-private"}}}}`)
	p, active, err := o.ReadProvider()
	if err != nil || !active || p.BaseURL != "" || p.HasToken {
		t.Fatalf("unknown keys treated as native fields: %+v, %v", p, err)
	}
}

func TestOpenCodeProviderFileSelectionAndCreation(t *testing.T) {
	o := NewOpenCode(t.TempDir())
	p := ProviderProfile{Name: "keyless", BaseURL: "http://localhost/v1", Model: "m"}
	if err := o.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got, _, err := o.ReadProvider(); err != nil || got.HasToken || got.EnvKey != "" {
		t.Fatalf("placeholder advertised as secret: %+v, %v", got, err)
	}
	if err := o.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(o.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("created config not removed")
	}
	jsonc := filepath.Join(o.configDir(), "opencode.jsonc")
	writeFile(t, jsonc, `{"autoupdate":false}`)
	if o.ProviderFile() != jsonc {
		t.Fatal("native JSONC filename ignored")
	}
	if err := o.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := o.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(o.configDir(), "opencode.json"), `{}`)
	if _, err := o.ValidateProvider(p); err == nil {
		t.Fatal("accepted ambiguous global files")
	}
	for _, bad := range []ProviderProfile{{BaseURL: p.BaseURL, Model: "m", WireAPI: "responses"}, {BaseURL: p.BaseURL, Model: "m", Token: "{file:private}"}, {BaseURL: p.BaseURL, Model: "{env:MODEL}"}} {
		if err := openCodeProviderFields(bad); err == nil || strings.Contains(err.Error(), "private") {
			t.Fatalf("unsafe profile: %v", err)
		}
	}
}
