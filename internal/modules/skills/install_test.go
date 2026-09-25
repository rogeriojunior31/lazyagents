package skills

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestDetectSource(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		in   string
		want Source
	}{
		{"https://github.com/x/y", SourceGit},
		{"http://github.com/x/y", SourceGit},
		{"git@github.com:x/y.git", SourceGit},
		{"x/y.git", SourceGit},
		{"usuario/repo-inexistente", SourceGit},
		{"/tmp/algo.zip", SourceZip},
		{"skills.ZIP", SourceZip},
		{dir, SourceDir},
		{"~/skills", SourceDir},
	}
	for _, tc := range tests {
		if got := DetectSource(tc.in); got != tc.want {
			t.Errorf("DetectSource(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestDiscoverDir(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	t.Run("SKILL.md at the root", func(t *testing.T) {
		root := writeSkill(t, t.TempDir(), "solo", validMD("solo", "only one"))
		found, origin, cleanup, err := svc.Discover(root)
		if err != nil || cleanup != "" {
			t.Fatalf("err=%v cleanup=%q", err, cleanup)
		}
		if len(found) != 1 || found[0].Name != "solo" || !found[0].Valid {
			t.Fatalf("found = %+v", found)
		}
		if origin.Type != "dir" || origin.Source != root {
			t.Errorf("origin = %+v", origin)
		}
	})

	t.Run("nested, with hidden dir dedupe", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, filepath.Join(root, "skills", "cat-a"), "alpha", validMD("alpha", "a"))
		writeSkill(t, filepath.Join(root, "skills", "cat-b"), "beta", validMD("beta", "b"))
		writeSkill(t, filepath.Join(root, ".openclaw", "skills"), "alpha", validMD("alpha", "hidden dup"))
		writeSkill(t, filepath.Join(root, "node_modules", "x"), "gamma", validMD("gamma", "ignored"))
		found, _, _, err := svc.Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 2 {
			t.Fatalf("want alpha and beta, got %+v", found)
		}
		if found[0].Name != "alpha" || found[0].Hidden {
			t.Errorf("alpha should be the non-hidden copy: %+v", found[0])
		}
		if found[0].Rel != filepath.Join("skills", "cat-a", "alpha") {
			t.Errorf("alpha Rel = %q", found[0].Rel)
		}
		if found[1].Name != "beta" {
			t.Errorf("second item: %+v", found[1])
		}
	})

	t.Run("no SKILL.md", func(t *testing.T) {
		if _, _, _, err := svc.Discover(t.TempDir()); err == nil {
			t.Fatal("should fail without SKILL.md")
		}
	})
}

func TestInstall(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	src1 := writeSkill(t, t.TempDir(), "new-one", validMD("new-one", "x"))
	src2 := writeSkill(t, t.TempDir(), "other", validMD("other", "y"))
	writeSkill(t, p.LibraryDir(), "new-one", validMD("new-one", "already there"))

	installed, err := svc.Install([]Found{
		{SrcDir: src1, Name: "new-one"},
		{SrcDir: src2, Name: "other", Rel: "sub/other"},
	}, Origin{Type: "git", Source: "https://github.com/x/y"})
	if err == nil {
		t.Fatal("want a partial error (new-one already exists)")
	}
	if len(installed) != 1 || installed[0] != "other" {
		t.Fatalf("installed = %v", installed)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "other", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	// recorded origin and round-trip through the scan
	o := readOrigin(filepath.Join(p.LibraryDir(), "other"))
	if o == nil || o.Type != "git" || o.Source != "https://github.com/x/y" ||
		o.Sub != "sub/other" || o.InstalledAt.IsZero() {
		t.Fatalf("origin = %+v", o)
	}
	sk := scanOne(t, svc, nil, "other")
	if sk.Origin == nil || sk.Origin.Source != "https://github.com/x/y" {
		t.Fatalf("scan did not expose the origin: %+v", sk.Origin)
	}
}

func TestDiscoverZip(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	zipPath := filepath.Join(t.TempDir(), "pack.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"skills/one/SKILL.md":  validMD("one", "from the zip"),
		"skills/one/extra.txt": "content",
		"../evil.txt":          "zip-slip",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	found, origin, cleanup, err := svc.Discover(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if origin.Type != "zip" || origin.Source != zipPath {
		t.Errorf("origin = %+v", origin)
	}
	defer os.RemoveAll(cleanup)
	if cleanup == "" {
		t.Fatal("zip should have a temp cleanup dir")
	}
	if len(found) != 1 || found[0].Name != "one" {
		t.Fatalf("found = %+v", found)
	}
	if _, err := os.Stat(filepath.Join(cleanup, "..", "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("zip-slip not blocked")
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "one", "extra.txt")); err != nil {
		t.Fatalf("extra file not installed: %v", err)
	}
}

// A repo with a skill and a plugin hook: both are discovered, and the hook goes
// to the hooks library, not the skills one.
func TestDiscoverAndInstallHooksFromRepo(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "code-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: code-review\ndescription: reviews diffs\n---\ninstructions.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	hookDir := filepath.Join(root, "plugins", "guard", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"description":"guard","hooks":{"Stop":[{"hooks":[{"type":"command","command":"bash \"${CLAUDE_PLUGIN_ROOT}/hooks/s.sh\""}]}]}}`
	if err := os.WriteFile(filepath.Join(hookDir, "hooks.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookDir, "s.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths := core.PathsIn(t.TempDir())
	svc := New(paths)
	found, origin, cleanup, err := svc.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if cleanup != "" {
		t.Errorf("a local folder should not make a temp dir: %q", cleanup)
	}
	var skills, hookItems int
	for _, f := range found {
		if f.Hook != nil {
			hookItems++
		} else {
			skills++
		}
	}
	if skills != 1 || hookItems != 1 {
		t.Fatalf("discovered %d skills, %d hooks: %+v", skills, hookItems, found)
	}

	names, err := svc.Install(found, origin)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("installed = %v", names)
	}
	if _, err := os.Stat(filepath.Join(paths.LibraryDir(), "code-review", "SKILL.md")); err != nil {
		t.Error("skill not installed")
	}
	entry, err := os.ReadFile(filepath.Join(paths.HooksDir(), "guard.json"))
	if err != nil {
		t.Fatalf("hook not imported: %v", err)
	}
	if !strings.Contains(string(entry), "export CLAUDE_PLUGIN_ROOT=") {
		t.Errorf("command not rewritten: %s", entry)
	}
	// Files keep their path relative to the plugin root.
	if _, err := os.Stat(filepath.Join(paths.HooksDir(), "guard", "hooks", "s.sh")); err != nil {
		t.Error("hook script not copied")
	}
	if _, err := os.Stat(filepath.Join(paths.LibraryDir(), "guard")); err == nil {
		t.Error("a hook must not become a library skill")
	}
}
