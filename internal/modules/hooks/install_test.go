package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Espelha o layout real de um plugin do marketplace do Claude Code.
const pluginHooks = `{
  "description": "avisos de segurança",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {"type": "command", "command": "python3 \"${CLAUDE_PLUGIN_ROOT}/hooks/pre.py\"", "timeout": 10}
        ]
      }
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "bash \"${CLAUDE_PLUGIN_ROOT}/hooks/stop.sh\""}]}
    ]
  }
}
`

func writeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "plugins", "security-guidance", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), []byte(pluginHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pre.py"), []byte("print('oi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stop.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Ruído que a varredura precisa ignorar.
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "hooks.json"), []byte(pluginHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscoverInFindsPluginHooks(t *testing.T) {
	found := DiscoverIn(writeRepo(t))
	if len(found) != 1 {
		t.Fatalf("DiscoverIn = %+v; queria 1 (o de .git não conta)", found)
	}
	f := found[0]
	if f.Plugin != "security-guidance" || f.Description != "avisos de segurança" {
		t.Errorf("found = %+v", f)
	}
	if len(f.Hooks) != 2 || f.Rel != "plugins/security-guidance/hooks" {
		t.Errorf("hooks = %+v, rel = %q", f.Hooks, f.Rel)
	}
	if ev := f.Events(); len(ev) != 2 || ev[0] != "PreToolUse" {
		t.Errorf("Events = %v", ev)
	}
}

func TestImportRewritesCommandAndCopiesScripts(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	found := DiscoverIn(writeRepo(t))
	names, err := Import(paths, found[0], "usuario/repo")
	if err != nil {
		t.Fatal(err)
	}
	// Um plugin vira UMA entrada, com os dois comandos dentro.
	if len(names) != 1 || names[0] != "security-guidance" {
		t.Fatalf("names = %v", names)
	}

	svc := New(nil, paths)
	lib, problems := svc.Library()
	if len(lib) != 1 || len(problems) != 0 {
		t.Fatalf("Library = %+v, %v", lib, problems)
	}
	entry := lib[0]
	dst := filepath.Join(paths.HooksDir(), "security-guidance")
	if len(entry.Hooks) != 2 || len(entry.Events()) != 2 {
		t.Fatalf("entrada = %+v", entry)
	}
	for _, h := range entry.Hooks {
		if strings.Contains(h.Command, "CLAUDE_PLUGIN_ROOT") {
			t.Errorf("variável do plugin sobrou: %q", h.Command)
		}
		if !strings.Contains(h.Command, dst) {
			t.Errorf("comando não aponta para a biblioteca: %q", h.Command)
		}
	}
	if !entry.Imported() || entry.Files != dst {
		t.Errorf("proveniência = %+v", entry)
	}
	if entry.Hooks[0].Matcher != "Edit|Write" {
		t.Errorf("matcher perdido: %+v", entry.Hooks[0])
	}

	// Scripts copiados, com o bit de execução preservado.
	if info, err := os.Stat(filepath.Join(dst, "stop.sh")); err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("stop.sh = %v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "pre.py")); err != nil {
		t.Error("pre.py não foi copiado")
	}

	// Reimportar por cima é erro explícito.
	if _, err := Import(paths, found[0], "usuario/repo"); err == nil {
		t.Error("reimportar deveria falhar")
	}

	// Apagar a entrada leva a pasta de scripts junto.
	if err := svc.Delete("security-guidance"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("scripts deveriam ter sido apagados com a última entrada")
	}
}

// Comando que aponta para fora de hooks/ é recusado: só essa pasta é copiada.
func TestImportRefusesCommandOutsideHooksDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plug", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"python3 \"${CLAUDE_PLUGIN_ROOT}/scripts/x.py\""}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	paths := core.PathsIn(t.TempDir())
	found := DiscoverIn(root)
	if len(found) != 1 {
		t.Fatalf("found = %+v", found)
	}
	_, err := Import(paths, found[0], "x")
	if err == nil || !strings.Contains(err.Error(), "fora de hooks/") {
		t.Fatalf("Import = %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.HooksDir(), "plug")); !os.IsNotExist(err) {
		t.Error("import recusado não pode deixar arquivo para trás")
	}
}
