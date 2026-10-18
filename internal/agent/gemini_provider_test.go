package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func geminiTestProfile() ProviderProfile {
	return ProviderProfile{Name: "fixture", BaseURL: "http://127.0.0.1:18765", Model: "gemini-2.5-pro", WireAPI: "gemini", Token: "fixture-private-key"}
}
func geminiTestHost(t *testing.T, settings, env string) *Gemini {
	t.Helper()
	g := NewGemini(t.TempDir())
	g.ProviderState = filepath.Join(t.TempDir(), "gemini-state.json")
	if settings != "" {
		writeFile(t, g.ProviderFile(), settings)
	}
	if env != "" {
		writeFile(t, g.providerEnvFile(), env)
	}
	return g
}
func readGeminiTest(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestGeminiProviderRestoresNativeAuthAndDotenv(t *testing.T) {
	for _, auth := range []string{"oauth-personal", "vertex-ai", geminiAPIAuth} {
		t.Run(auth, func(t *testing.T) {
			config := `{"foreign":{"keep":true},"model":{"name":"native-model"},"security":{"auth":{"selectedType":"` + auth + `","foreign":"keep"},"folderTrust":{"enabled":true}}}`
			env := "# Keep this comment\r\nOTHER='untouched # value'\r\nGEMINI_API_KEY='native-private-key'\nGOOGLE_GEMINI_BASE_URL=https://native.example\nGEMINI_MODEL=original-model"
			g := geminiTestHost(t, config, env)
			login := filepath.Join(g.configDir(), "oauth_creds.json")
			writeFile(t, login, "do-not-read-or-touch")
			p := geminiTestProfile()
			before := readGeminiTest(t, g.ProviderFile())
			preview, err := g.PreviewProvider(&p)
			if err != nil || len(preview.Files) != 2 {
				t.Fatalf("preview: %+v %v", preview, err)
			}
			encoded, _ := json.Marshal(preview)
			for _, secret := range []string{p.Token, "native-private-key", "untouched # value"} {
				if bytes.Contains(encoded, []byte(secret)) {
					t.Fatal("preview leaks private content")
				}
			}
			if !bytes.Equal(before, readGeminiTest(t, g.ProviderFile())) || string(readGeminiTest(t, g.providerEnvFile())) != env {
				t.Fatal("preview wrote files")
			}
			backups := t.TempDir()
			if err := g.ApplyProvider(p, backups); err != nil {
				t.Fatal(err)
			}
			live, active, err := g.ReadProvider()
			if err != nil || !active || live.BaseURL != p.BaseURL || live.Model != p.Model || live.Token != "" || !live.HasToken {
				t.Fatalf("read: %+v %v %v", live, active, err)
			}
			st := readGeminiTest(t, g.ProviderState)
			if bytes.Contains(st, []byte(p.Token)) || bytes.Contains(st, []byte("native-private-key")) {
				t.Fatal("ownership leaks credentials")
			}
			if !bytes.HasPrefix(readGeminiTest(t, g.providerEnvFile()), []byte(env)) {
				t.Fatal("foreign dotenv changed")
			}
			info, _ := os.Stat(g.providerEnvFile())
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Fatal("insecure dotenv")
			}
			for _, pattern := range []string{".env.*", "settings.json.*"} {
				files, _ := filepath.Glob(filepath.Join(backups, pattern))
				if len(files) == 0 {
					t.Fatal("missing backup")
				}
				for _, f := range files {
					info, _ := os.Stat(f)
					if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
						t.Fatal("insecure backup")
					}
				}
			}
			// A new process must restore the public auth selection and exact foreign dotenv.
			restored := NewGemini(g.Home)
			restored.ProviderState = g.ProviderState
			if err := restored.ClearProvider(backups); err != nil {
				t.Fatal(err)
			}
			var a, b any
			_ = json.Unmarshal(before, &a)
			_ = json.Unmarshal(readGeminiTest(t, g.ProviderFile()), &b)
			if !reflect.DeepEqual(a, b) || string(readGeminiTest(t, g.providerEnvFile())) != env || string(readGeminiTest(t, login)) != "do-not-read-or-touch" {
				t.Fatal("clear changed native settings, dotenv or login")
			}
		})
	}
}
func TestGeminiProviderOwnershipAndExternalEdits(t *testing.T) {
	g := geminiTestHost(t, "", "")
	p := geminiTestProfile()
	if err := g.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	p.Model = "gemini-2.0-flash"
	if err := g.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	config := `{"security":{"auth":{"selectedType":"oauth-personal","externalField":true}},"foreign":42}`
	writeFile(t, g.ProviderFile(), config)
	// Unrelated dotenv additions before the block survive; changing the block fails closed.
	env := readGeminiTest(t, g.providerEnvFile())
	writeFile(t, g.providerEnvFile(), "NEW=value\n"+string(env))
	if err := g.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if string(readGeminiTest(t, g.providerEnvFile())) != "NEW=value\n" {
		t.Fatal("foreign addition lost")
	}
	data := readGeminiTest(t, g.ProviderFile())
	if !bytes.Contains(data, []byte("oauth-personal")) || !bytes.Contains(data, []byte("externalField")) {
		t.Fatal("external auth lost")
	}
	if err := g.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	env = readGeminiTest(t, g.providerEnvFile())
	writeFile(t, g.providerEnvFile(), strings.ReplaceAll(string(env), p.Model, "changed-model"))
	before := readGeminiTest(t, g.ProviderFile())
	if err := g.ClearProvider(t.TempDir()); err == nil {
		t.Fatal("cleared edited block")
	}
	if !bytes.Equal(before, readGeminiTest(t, g.ProviderFile())) {
		t.Fatal("failed clear wrote settings")
	}
}
func TestGeminiProviderCreatedFilesAndNativeNoop(t *testing.T) {
	g := geminiTestHost(t, "", "")
	p := geminiTestProfile()
	if err := g.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := g.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for _, path := range ProviderFiles(g) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("created file remains: %s", path)
		}
	}
	g = geminiTestHost(t, "{\"foreign\": 42}", "OTHER=x\n")
	before := readGeminiTest(t, g.ProviderFile())
	if err := g.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, readGeminiTest(t, g.ProviderFile())) {
		t.Fatal("native clear rewrote unmanaged file")
	}
}
func TestGeminiProviderPreflightAndRedaction(t *testing.T) {
	for _, bad := range []string{`{"security":[]}`, `{"security":{"auth":{"selectedType":null}}}`, `{"security":{"auth":{"selectedType":"unknown-private-value"}}}`, `{"security":{"auth":{"enforcedType":"oauth-personal"}}}`, `{"security":{"auth":{"useExternal":true}}}`, `{"security":{},"security":{}}`, `{"security":{"auth":{"selectedType":true}}}`, `{} trailing-private-value`} {
		g := geminiTestHost(t, bad, "")
		before := readGeminiTest(t, g.ProviderFile())
		p := geminiTestProfile()
		if _, err := g.ValidateProvider(p); err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("validation: %v", err)
		}
		if err := g.ApplyProvider(p, t.TempDir()); err == nil {
			t.Fatal("applied invalid settings")
		}
		if !bytes.Equal(before, readGeminiTest(t, g.ProviderFile())) {
			t.Fatal("preflight wrote")
		}
	}
	for _, env := range []string{"KEY='multiline\nprivate-value'", "bad-private-value", "GEMINI_API_KEY_AUTH_MECHANISM=bearer", "GOOGLE_GENAI_USE_VERTEXAI=true", "# lazyagents provider start\nKEY=x\n"} {
		g := geminiTestHost(t, `{}`, env)
		p := geminiTestProfile()
		if _, err := g.ValidateProvider(p); err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("dotenv validation: %v", err)
		}
	}
	for _, edit := range []func(*ProviderProfile){func(p *ProviderProfile) { p.WireAPI = "chat" }, func(p *ProviderProfile) { p.EnvKey = "OTHER" }, func(p *ProviderProfile) { p.Token = "" }, func(p *ProviderProfile) { p.BaseURL = "http://remote.example" }, func(p *ProviderProfile) { p.BaseURL = "https://example.test/?key=private-value" }, func(p *ProviderProfile) { p.Model = "" }, func(p *ProviderProfile) { p.Token = "private-value'\ninjected=x" }} {
		p := geminiTestProfile()
		edit(&p)
		g := geminiTestHost(t, "", "")
		if _, err := g.ValidateProvider(p); err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("profile validation: %v", err)
		}
	}
	g := geminiTestHost(t, "", "")
	t.Setenv("GEMINI_API_KEY_AUTH_MECHANISM", "bearer")
	if _, err := g.ValidateProvider(geminiTestProfile()); err == nil {
		t.Fatal("accepted bearer override")
	}
}
func TestGeminiEnvironmentAndRecoveryHideValues(t *testing.T) {
	g := geminiTestHost(t, "", "")
	p := geminiTestProfile()
	p.Token, p.EnvKey = "", "GEMINI_API_KEY"
	if err := g.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GEMINI_API_KEY", "private-environment-value")
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "https://private-environment-value.example")
	diagnostics, err := g.ProviderEnvironment()
	if err != nil || len(diagnostics) < 2 {
		t.Fatalf("diagnostics: %+v %v", diagnostics, err)
	}
	data, _ := json.Marshal(diagnostics)
	if bytes.Contains(data, []byte("private-environment-value")) {
		t.Fatal("diagnostics leaked values")
	}
	fields := g.ProviderFields(p)
	if fields["protocol"] != "gemini" || fields["credential"] != "env $GEMINI_API_KEY" {
		t.Fatal(fields)
	}
	files := g.RecoveryFiles()
	if len(files) != 2 {
		t.Fatal("missing recovery destinations")
	}
	diff, err := files[1].Changes([]byte("GEMINI_API_KEY=private-before"), []byte("GEMINI_API_KEY=private-after"))
	if err != nil || strings.Contains(strings.Join(diff, " "), "private-") {
		t.Fatalf("diff: %v %v", diff, err)
	}
}

func TestGeminiPartialWriteKeepsOwnershipAndConcurrentSettings(t *testing.T) {
	g := geminiTestHost(t, `{"security":{"auth":{"selectedType":"oauth-personal"}}}`, "OTHER=keep\n")
	p := geminiTestProfile()
	geminiBeforeSettingsSave = func() {
		writeFile(t, g.ProviderFile(), `{"security":{"auth":{"selectedType":"oauth-personal"}},"nativeEdit":true}`)
	}
	t.Cleanup(func() { geminiBeforeSettingsSave = func() {} })
	if err := g.ApplyProvider(p, t.TempDir()); err == nil || !strings.Contains(err.Error(), "dotenv was updated") {
		t.Fatalf("partial write: %v", err)
	}
	if !bytes.Contains(readGeminiTest(t, g.providerEnvFile()), []byte(p.Token)) {
		t.Fatal("did not exercise partial write")
	}
	geminiBeforeSettingsSave = func() {}
	restarted := NewGemini(g.Home)
	restarted.ProviderState = g.ProviderState
	if err := restarted.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if string(readGeminiTest(t, g.providerEnvFile())) != "OTHER=keep\n" || !bytes.Contains(readGeminiTest(t, g.ProviderFile()), []byte("nativeEdit")) {
		t.Fatal("partial cleanup discarded foreign content")
	}
}

func TestRecordedGeminiProviderRoundTrip(t *testing.T) {
	type snapshot struct {
		Settings json.RawMessage
		Env      string
	}
	var proof struct {
		Version, OS, Arch, Recorded, How string
		Before                           snapshot
		Probes                           []struct {
			Credential                 string
			Applied, Cleared           snapshot
			AppliedModel, ClearedModel string
		}
	}
	data := readGeminiTest(t, filepath.Join(fixturesDir, "gemini-cli", "0.64.0", "provider-roundtrip.json"))
	if json.Unmarshal(data, &proof) != nil || proof.Version != "0.64.0" || proof.OS != "linux" || proof.Arch != "amd64" || proof.Recorded == "" || proof.How == "" || len(proof.Probes) != 2 {
		t.Fatal("incomplete native Gemini evidence")
	}
	for _, probe := range proof.Probes {
		t.Run(probe.Credential, func(t *testing.T) {
			g := geminiTestHost(t, string(proof.Before.Settings), proof.Before.Env)
			p := geminiTestProfile()
			p.Token = "fixture-managed-key"
			if probe.Credential == "environment" {
				p.Token = ""
				p.EnvKey = "GEMINI_API_KEY"
				p.Model = "gemini-2.0-flash"
			} else if probe.Credential != "token" {
				t.Fatal("unknown credential case")
			}
			if probe.AppliedModel != p.Model || probe.ClearedModel != "gemini-2.5-flash" {
				t.Fatal("native inference used wrong model")
			}
			assertSnapshot := func(want snapshot) {
				t.Helper()
				a, e := recoveryJSONFields(readGeminiTest(t, g.ProviderFile()))
				b, f := recoveryJSONFields(want.Settings)
				if e != nil || f != nil || !reflect.DeepEqual(a, b) || string(readGeminiTest(t, g.providerEnvFile())) != want.Env {
					t.Fatal("adapter no longer matches CLI-verified inputs")
				}
			}
			if err := g.ApplyProvider(p, t.TempDir()); err != nil {
				t.Fatal(err)
			}
			assertSnapshot(probe.Applied)
			got, active, err := g.ReadProvider()
			if err != nil || !active || got.EnvKey != p.EnvKey || got.HasToken != (p.Token != "") {
				t.Fatalf("readback: %+v %v", got, err)
			}
			if err := g.ClearProvider(t.TempDir()); err != nil {
				t.Fatal(err)
			}
			assertSnapshot(probe.Cleared)
			assertSnapshot(proof.Before)
		})
	}
}

func TestGeminiFailedBackupCanClearWithoutCreatingNativeFiles(t *testing.T) {
	g := geminiTestHost(t, "", "")
	p := geminiTestProfile()
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	writeFile(t, blocked, "block")
	// Native files must exist for Backup to attempt its destination.
	writeFile(t, g.providerEnvFile(), "OTHER=keep\n")
	if err := g.ApplyProvider(p, blocked); err == nil {
		t.Fatal("expected backup failure")
	}
	if err := g.ClearProvider(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(g.ProviderFile()); !os.IsNotExist(err) {
		t.Fatal("clear created settings after a failed apply")
	}
	if string(readGeminiTest(t, g.providerEnvFile())) != "OTHER=keep\n" {
		t.Fatal("clear altered native dotenv")
	}
}

func TestGeminiOwnershipCannotCrossHomes(t *testing.T) {
	g := geminiTestHost(t, "", "")
	p := geminiTestProfile()
	if err := g.ApplyProvider(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	t.Setenv("GEMINI_CLI_HOME", other)
	writeFile(t, g.ProviderFile(), `{"security":{"auth":{"selectedType":"gemini-api-key"}}}`)
	foreign := readGeminiTest(t, g.ProviderFile())
	if err := g.ClearProvider(t.TempDir()); err == nil {
		t.Fatal("cleared another home's authentication")
	}
	if err := g.ApplyProvider(p, t.TempDir()); err == nil {
		t.Fatal("abandoned ownership from another home")
	}
	if !bytes.Equal(foreign, readGeminiTest(t, g.ProviderFile())) {
		t.Fatal("another home changed")
	}
}

func TestGeminiProviderRejectsAmbiguousHomeOverride(t *testing.T) {
	g := geminiTestHost(t, "", "")
	for _, value := range []string{"relative/home", "~/literal-home"} {
		t.Setenv("GEMINI_CLI_HOME", value)
		if _, err := g.ValidateProvider(geminiTestProfile()); err == nil {
			t.Fatal("accepted nonabsolute native home")
		}
		if err := g.ClearProvider(t.TempDir()); err == nil {
			t.Fatal("cleared unresolved native home")
		}
	}
}
