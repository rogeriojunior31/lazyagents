package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Mirrors the real layout of a Claude Code marketplace plugin.
const pluginHooks = `{
  "description": "security warnings",
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
	if err := os.WriteFile(filepath.Join(dir, "pre.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stop.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Noise the scan must ignore.
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "hooks.json"), []byte(pluginHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscoverInFindsPluginHooks(t *testing.T) {
	found := DiscoverIn(writeRepo(t), "")
	if len(found) != 1 {
		t.Fatalf("DiscoverIn = %+v; want 1 (the one in .git does not count)", found)
	}
	f := found[0]
	if f.Plugin != "security-guidance" || f.Description != "security warnings" {
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
	found := DiscoverIn(writeRepo(t), "")
	names, err := Import(paths, found[0], "user/repo")
	if err != nil {
		t.Fatal(err)
	}
	// One plugin becomes ONE entry holding both commands.
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
		t.Fatalf("entry = %+v", entry)
	}
	for _, h := range entry.Hooks {
		if !strings.HasPrefix(h.Command, "export CLAUDE_PLUGIN_ROOT="+shellQuote(dst)+"; ") {
			t.Errorf("command does not set the copy root: %q", h.Command)
		}
	}
	if !entry.Imported() || entry.Files != dst {
		t.Errorf("origin = %+v", entry)
	}
	if entry.Hooks[0].Matcher != "Edit|Write" {
		t.Errorf("matcher lost: %+v", entry.Hooks[0])
	}

	// Scripts keep their path under the plugin root and their exec bit.
	if info, err := os.Stat(filepath.Join(dst, "hooks", "stop.sh")); err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("stop.sh = %v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "hooks", "pre.py")); err != nil {
		t.Error("pre.py was not copied")
	}

	// Re-importing over an entry is an explicit error.
	if _, err := Import(paths, found[0], "user/repo"); err == nil {
		t.Error("re-import should fail")
	}

	// Deleting the entry removes its scripts folder.
	if err := svc.Delete("security-guidance"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("scripts should be deleted with the last entry")
	}
}

// A command pointing at a missing folder is rejected.
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
	found := DiscoverIn(root, "my-repo")
	if len(found) != 1 {
		t.Fatalf("found = %+v", found)
	}
	_, err := Import(paths, found[0], "x")
	if err == nil || !strings.Contains(err.Error(), "does not exist in the source") {
		t.Fatalf("Import = %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.HooksDir(), "plug")); !os.IsNotExist(err) {
		t.Error("a rejected import must leave no files behind")
	}
}

// dog_stack layout: hooks/ at the repo root and commands citing another root
// folder (scripts/). Both are copied keeping the layout.
func TestImportCopiesEveryReferencedDir(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"hooks", "scripts/hooks", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	doc := `{"hooks":{
      "SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"bash \"${CLAUDE_PLUGIN_ROOT}/hooks/session-start.sh\"","async":true,"timeout":10}]}],
      "PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"bash \"${CLAUDE_PLUGIN_ROOT}/scripts/hooks/run.sh\" \"flag\""}]}]
    }}`
	write := func(rel, body string, perm os.FileMode) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), perm); err != nil {
			t.Fatal(err)
		}
	}
	write("hooks/hooks.json", doc, 0o644)
	write("hooks/session-start.sh", "#!/bin/sh\n", 0o755)
	write("scripts/hooks/run.sh", "#!/bin/sh\n", 0o755)
	write("docs/index.md", "# unrelated\n", 0o644)

	paths := core.PathsIn(t.TempDir())
	found := DiscoverIn(root, "dog_stack")
	if len(found) != 1 || found[0].Plugin != "dog_stack" {
		t.Fatalf("found = %+v; the name comes from the repo, not the temp dir", found)
	}
	if _, err := Import(paths, found[0], "user/dog_stack"); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(paths.HooksDir(), "dog_stack")
	for _, rel := range []string{"hooks/session-start.sh", "scripts/hooks/run.sh"} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("%s was not copied", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "docs")); !os.IsNotExist(err) {
		t.Error("docs/ is not referenced by the commands and should not be copied")
	}

	svc := New(nil, paths)
	entry, err := svc.Get("dog_stack")
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Hooks) != 2 {
		t.Fatalf("entry = %+v", entry)
	}
	for _, h := range entry.Hooks {
		if !strings.HasPrefix(h.Command, "export CLAUDE_PLUGIN_ROOT="+shellQuote(dst)+"; ") {
			t.Errorf("root not exported: %q", h.Command)
		}
	}
	if !entry.Hooks[0].Async {
		t.Errorf("async lost: %+v", entry.Hooks[0]) // without it the hook blocks
	}
	if entry.Hooks[0].Timeout != 10 || entry.Hooks[1].Matcher != "Edit" {
		t.Errorf("fields lost: %+v", entry.Hooks)
	}
}

func TestRewriteCommandPreservesShellQuoting(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	root := filepath.Join(t.TempDir(), "space '\"$(false)")
	cmd, err := rewriteCommand(`printf '%s' "${CLAUDE_PLUGIN_ROOT}"`, root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil || string(out) != root {
		t.Fatalf("%q: %v, want %q", out, err, root)
	}
}

func TestImportDoesNotOverwriteLibraryEntry(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	found := DiscoverIn(writeRepo(t), "")[0]
	svc := New(nil, paths)
	h := Hook{Name: found.Plugin, Hooks: []agent.Hook{{Event: "Stop", Command: "original"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(paths, found, "repo"); err == nil {
		t.Fatal("overwrote hook")
	}
	got, err := svc.Get(h.Name)
	if err != nil || got.Hooks[0].Command != "original" {
		t.Fatalf("%+v %v", got, err)
	}
}

// Claude Code rejects settings.json commands with a literal ${CLAUDE_PLUGIN_ROOT};
// the copy uses the unbraced form.
func TestRewriteCommandAvoidsBracedPluginRoot(t *testing.T) {
	cmd, err := rewriteCommand(`bash "${CLAUDE_PLUGIN_ROOT}/hooks/a.sh" "${CLAUDE_PLUGIN_ROOT}/b"`, "/lib/p")
	if err != nil {
		t.Fatal(err)
	}
	want := `export CLAUDE_PLUGIN_ROOT='/lib/p'; bash "$CLAUDE_PLUGIN_ROOT/hooks/a.sh" "$CLAUDE_PLUGIN_ROOT/b"`
	if cmd != want {
		t.Errorf("got  %q\nwant %q", cmd, want)
	}
	if _, err := rewriteCommand(`echo ${CLAUDE_PLUGIN_ROOT}x`, "/lib/p"); err == nil {
		t.Error("a variable glued to a name should be rejected")
	}
}

// An entry imported with the old form is fixed in the library and in
// settings.json, leaving the foreign hook alone.
func TestRepairImported(t *testing.T) {
	svc, home := testService(t)
	old := agent.Hook{Event: agent.HookStop, Matcher: "*",
		Command: `export CLAUDE_PLUGIN_ROOT='/lib/p'; bash "${CLAUDE_PLUGIN_ROOT}/hooks/stop.sh"`}
	foreign := agent.Hook{Event: agent.HookStop, Command: "echo mine"}
	if err := svc.Save(Hook{Name: "p", Source: "repo · p", Files: "/lib/p", Hooks: []agent.Hook{old}}); err != nil {
		t.Fatal(err)
	}
	claude := agent.NewClaude(home)
	for _, h := range []agent.Hook{foreign, old} {
		if err := claude.AddHook(h, svc.backupsDir); err != nil {
			t.Fatal(err)
		}
	}

	names, err := svc.RepairImported()
	if err != nil || len(names) != 1 || names[0] != "p" {
		t.Fatalf("RepairImported = %v, %v", names, err)
	}
	fixed := old
	fixed.Command = `export CLAUDE_PLUGIN_ROOT='/lib/p'; bash "$CLAUDE_PLUGIN_ROOT/hooks/stop.sh"`
	got, _ := svc.Get("p")
	if got.Hooks[0].Command != fixed.Command {
		t.Errorf("library = %q", got.Hooks[0].Command)
	}
	installed, _ := claude.ReadHooks()
	if len(installed) != 2 || !containsHook(installed, fixed) || !containsHook(installed, foreign) {
		t.Errorf("installed = %+v", installed)
	}

	// Second pass: nothing to do.
	if names, err := svc.RepairImported(); err != nil || len(names) != 0 {
		t.Errorf("repeated = %v, %v", names, err)
	}
}

// The root-export prefix is neither checked as the executable nor shown.
func TestRootExportIsTransparent(t *testing.T) {
	cmd := `export CLAUDE_PLUGIN_ROOT='/lib/p'; sh "$CLAUDE_PLUGIN_ROOT/hooks/a.sh"`
	if p := commandProblem(cmd); p != "" {
		t.Errorf("commandProblem = %q", p)
	}
	if got := displayCommand(cmd); got != `sh "./hooks/a.sh"` {
		t.Errorf("displayCommand = %q", got)
	}
}
