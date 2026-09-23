package agent

import (
	"os"
	"path/filepath"
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
  "env": {"MEU_VAR": "1"},
  "hooks": {"SessionStart": []}
}
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := c.ReadProvider(); ok || err != nil {
		t.Fatalf("ReadProvider sem provedor = %v, %v; queria false, nil", ok, err)
	}

	p := ProviderProfile{Name: "meu", BaseURL: "https://exemplo/api", Token: "segredo-abc", Model: "sonnet"}
	backups := filepath.Join(home, "backups")
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"hooks"`, `"MEU_VAR"`, "https://exemplo/api", "segredo-abc"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("settings.json sem %q:\n%s", want, data)
		}
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("permissão = %v, queria 0600", info.Mode().Perm())
	}

	got, ok, err := c.ReadProvider()
	if err != nil || !ok {
		t.Fatalf("ReadProvider = %v, %v", ok, err)
	}
	if got.BaseURL != p.BaseURL || got.Model != p.Model {
		t.Errorf("ReadProvider = %+v", got)
	}
	if got.Token != "" || !got.HasToken {
		t.Errorf("token vazou na leitura: %+v", got)
	}

	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "segredo-abc") || strings.Contains(string(data), "exemplo") {
		t.Errorf("clear não limpou:\n%s", data)
	}
	for _, want := range []string{`"hooks"`, `"MEU_VAR"`, `"model"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("clear comeu %q:\n%s", want, data)
		}
	}
	if _, ok, _ := c.ReadProvider(); ok {
		t.Error("ReadProvider depois do clear devia ser false")
	}
}

const codexLiveTOML = `model = "gpt-6"
model_reasoning_effort = "medium"

[projects."/home/eu/Projects"]
trust_level = "trusted"

[hooks.state."/home/eu/.codex/hooks.json:session_start:0:0"]
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

	p := ProviderProfile{Name: "local", BaseURL: "http://localhost:11434/v1", EnvKey: "MINHA_CHAVE", Model: "qwen", WireAPI: "responses"}
	backups := filepath.Join(home, "backups")
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`trusted_hash = "sha256:abc"`, `[projects."/home/eu/Projects"]`, `model_reasoning_effort = "medium"`} {
		if !strings.Contains(text, want) {
			t.Errorf("apply comeu %q:\n%s", want, text)
		}
	}
	// A chave de topo tem que entrar ANTES da primeira tabela, senão vira
	// chave da tabela anterior.
	if i, j := strings.Index(text, "model_provider ="), strings.Index(text, "[projects."); i < 0 || i > j {
		t.Errorf("model_provider fora do topo:\n%s", text)
	}
	if !strings.Contains(text, "[model_providers.lazyagents]") || !strings.Contains(text, `env_key = "MINHA_CHAVE"`) {
		t.Errorf("tabela do provider faltando:\n%s", text)
	}

	got, ok, err := c.ReadProvider()
	if err != nil || !ok {
		t.Fatalf("ReadProvider = %v, %v", ok, err)
	}
	if got.BaseURL != p.BaseURL || got.Model != "qwen" || got.EnvKey != "MINHA_CHAVE" || got.WireAPI != "responses" || got.Name != "local" {
		t.Errorf("ReadProvider = %+v", got)
	}

	// Enquanto o perfil está aplicado, o model do perfil é o único da raiz.
	if n := strings.Count(text, "\nmodel = "); n != 1 {
		t.Errorf("chave model repetida na raiz (%d):\n%s", n, text)
	}
	if !strings.Contains(text, `model = "qwen"`) || !strings.Contains(text, codexPrevModel+`"gpt-6"`) {
		t.Errorf("troca de model não guardou o anterior:\n%s", text)
	}

	// Reaplicar não duplica bloco.
	if err := c.ApplyProvider(p, backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if n := strings.Count(string(data), "[model_providers.lazyagents]"); n != 1 {
		t.Errorf("blocos duplicados: %d\n%s", n, data)
	}

	if err := c.ClearProvider(backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	text = string(data)
	if strings.Contains(text, "lazyagents") {
		t.Errorf("clear deixou bloco gerenciado:\n%s", text)
	}
	// O model do usuário volta, uma vez só (chave repetida quebraria o TOML).
	if n := strings.Count(text, `model = "gpt-6"`); n != 1 {
		t.Errorf("model original voltou %d vez(es):\n%s", n, text)
	}
	for _, want := range []string{`trusted_hash = "sha256:abc"`, `model_reasoning_effort = "medium"`, `[projects."/home/eu/Projects"]`} {
		if !strings.Contains(text, want) {
			t.Errorf("clear comeu %q:\n%s", want, text)
		}
	}
	if _, ok, _ := c.ReadProvider(); ok {
		t.Error("ReadProvider depois do clear devia ser false")
	}
}

func TestCodexProviderRefusesTokenWithoutEnvKey(t *testing.T) {
	c := NewCodex(t.TempDir())
	err := c.ApplyProvider(ProviderProfile{Name: "x", BaseURL: "https://e", Token: "segredo"}, "")
	if err == nil {
		t.Fatal("queria erro: o Codex não guarda token no config.toml")
	}
	if strings.Contains(err.Error(), "segredo") {
		t.Errorf("token vazou no erro: %v", err)
	}
	if err := c.ApplyProvider(ProviderProfile{Name: "x"}, ""); err == nil {
		t.Error("queria erro sem baseUrl")
	}
}

func TestCodexProviderReadsProviderSetToMao(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	const manual = `model_provider = "ollama"
model = "qwen3"

[model_providers.ollama]
name = "Ollama"
base_url = "http://localhost:11434/v1"  # comentário
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
	p := ProviderProfile{Name: "x", Token: "segredo"}.Redacted()
	if p.Token != "" || !p.HasToken {
		t.Errorf("Redacted = %+v", p)
	}
	if again := p.Redacted(); !again.HasToken { // idempotente
		t.Errorf("Redacted duas vezes perdeu HasToken: %+v", again)
	}
	got := ProviderProfile{Name: "x"}.Redacted()
	if got.HasToken {
		t.Errorf("HasToken sem token: %+v", got)
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
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
}

func TestCodexRejectsUnsupportedWireAPI(t *testing.T) {
	if err := NewCodex(t.TempDir()).ApplyProvider(ProviderProfile{BaseURL: "https://example.com", WireAPI: "chat"}, ""); err == nil {
		t.Fatal("accepted removed protocol")
	}
}

func TestCodexRefusesMultilineTOML(t *testing.T) {
	c := NewCodex(t.TempDir())
	original := "instructions = \"\"\"\nkeep this\n\"\"\"\n"
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://example.com"}, ""); err == nil {
		t.Fatal("accepted unsupported multiline TOML")
	}
	data, _ := os.ReadFile(c.ProviderFile())
	if string(data) != original {
		t.Fatal("modified config")
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
