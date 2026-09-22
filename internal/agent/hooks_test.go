package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Arquivo vivo com um hook do usuário, um campo que o lazyagents não conhece
// e uma entrada de tipo desconhecido — nada disso pode se perder.
const liveHooks = `{
  "model": "opus",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "^(startup|resume)$",
        "enabled": true,
        "hooks": [
          {"type": "command", "command": "bash meu.sh", "timeout": 10}
        ]
      }
    ],
    "PreToolUse": [
      {"hooks": [{"type": "mcp", "command": "nada"}]}
    ]
  }
}
`

func writeLive(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(liveHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeHooksAddRemovePreservesForeign(t *testing.T) {
	home := t.TempDir()
	path := writeLive(t, home)
	c := NewClaude(home)
	backups := filepath.Join(home, "backups")

	got, err := c.ReadHooks()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Command != "bash meu.sh" || got[0].Matcher != "^(startup|resume)$" || got[0].Timeout != 10 {
		t.Fatalf("ReadHooks = %+v", got) // a entrada "mcp" não é comando: fica de fora da leitura
	}

	mine := Hook{Event: HookSessionStart, Command: "lazyagents doctor", Timeout: 5}
	if err := c.AddHook(mine, backups); err != nil {
		t.Fatal(err)
	}
	if err := c.AddHook(mine, backups); err != nil { // idempotente
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if n := strings.Count(text, "lazyagents doctor"); n != 1 {
		t.Errorf("hook instalado %d vez(es):\n%s", n, text)
	}
	for _, want := range []string{`"model": "opus"`, `"enabled": true`, `"mcp"`, "bash meu.sh"} {
		if !strings.Contains(text, want) {
			t.Errorf("add comeu %q:\n%s", want, text)
		}
	}
	if !json.Valid(data) {
		t.Fatalf("JSON inválido:\n%s", text)
	}

	if err := c.RemoveHook(mine, backups); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "lazyagents doctor") {
		t.Errorf("remove não tirou:\n%s", data)
	}
	// Volta ao conteúdo original (a formatação é normalizada pelo primitivo,
	// então a comparação é semântica).
	var before, after any
	_ = json.Unmarshal([]byte(liveHooks), &before)
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("add+remove não devolveu o arquivo ao original:\n%s", data)
	}
}

// Remover o último hook de um evento tira o evento; remover o último de todos
// tira a chave "hooks" — sem deixar lixo onde não havia nada.
func TestHooksCleanupWhenEmpty(t *testing.T) {
	home := t.TempDir()
	c := NewClaude(home)
	h := Hook{Event: HookStop, Command: "echo fim"}
	if err := c.AddHook(h, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(c.HooksFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "echo fim") {
		t.Fatalf("hook não foi gravado:\n%s", data)
	}
	if err := c.RemoveHook(h, ""); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(c.HooksFile())
	if strings.Contains(string(data), "hooks") {
		t.Errorf("chave hooks deveria ter sumido:\n%s", data)
	}
}

// Hook do lazyagents dentro de um grupo que o usuário compartilhou com outro
// comando: sai só o nosso.
func TestRemoveFromSharedGroup(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	shared := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"dele.sh"},{"type":"command","command":"meu.sh"}]}]}}`
	if err := os.WriteFile(path, []byte(shared), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewClaude(home)
	if err := c.RemoveHook(Hook{Event: HookStop, Command: "meu.sh"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "meu.sh") || !strings.Contains(string(data), "dele.sh") {
		t.Errorf("remoção em grupo compartilhado errada:\n%s", data)
	}
}

func TestCodexHooksFileAndNote(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	if filepath.Base(c.HooksFile()) != "hooks.json" {
		t.Errorf("HooksFile = %s", c.HooksFile())
	}
	// Sem config.toml: o recurso está desligado.
	if note := c.HooksNote(); !strings.Contains(note, "desligados") {
		t.Errorf("nota = %q", note)
	}
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte("[features]\nhooks = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if note := c.HooksNote(); !strings.Contains(note, "confian") {
		t.Errorf("com hooks ligados a nota devia falar de confiança: %q", note)
	}

	h := Hook{Event: HookSessionStart, Command: "echo oi", Timeout: 5}
	if err := c.AddHook(h, ""); err != nil {
		t.Fatal(err)
	}
	got, err := c.ReadHooks()
	if err != nil || len(got) != 1 || !got[0].Same(h) {
		t.Fatalf("ReadHooks = %+v, %v", got, err)
	}
	// O lazyagents não mexe no estado de confiança nem liga o recurso.
	cfg, _ := os.ReadFile(c.ProviderFile())
	if strings.Contains(string(cfg), "trusted_hash") {
		t.Errorf("config.toml foi tocado:\n%s", cfg)
	}
}

// Evento gravado com outra caixa não pode virar chave duplicada.
func TestEventKeyIsCaseInsensitive(t *testing.T) {
	home := t.TempDir()
	c := NewCodex(home)
	if err := os.MkdirAll(c.configDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.HooksFile(), []byte(`{"hooks":{"session_start":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.AddHook(Hook{Event: HookSessionStart, Command: "x.sh"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.HooksFile())
	if strings.Contains(string(data), "SessionStart") {
		t.Errorf("criou chave duplicada:\n%s", data)
	}
	got, _ := c.ReadHooks()
	if len(got) != 1 {
		t.Errorf("ReadHooks = %+v", got)
	}
}
