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
			t.Errorf("DetectSource(%q) = %v, quer %v", tc.in, got, tc.want)
		}
	}
}

func TestDiscoverDir(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	t.Run("SKILL.md na raiz", func(t *testing.T) {
		root := writeSkill(t, t.TempDir(), "solo", validMD("solo", "única"))
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

	t.Run("aninhado com dedupe de dir oculto", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, filepath.Join(root, "skills", "cat-a"), "alpha", validMD("alpha", "a"))
		writeSkill(t, filepath.Join(root, "skills", "cat-b"), "beta", validMD("beta", "b"))
		writeSkill(t, filepath.Join(root, ".openclaw", "skills"), "alpha", validMD("alpha", "dup oculta"))
		writeSkill(t, filepath.Join(root, "node_modules", "x"), "gamma", validMD("gamma", "ignorada"))
		found, _, _, err := svc.Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 2 {
			t.Fatalf("esperava alpha e beta, veio %+v", found)
		}
		if found[0].Name != "alpha" || found[0].Hidden {
			t.Errorf("alpha deveria ser a versão não-oculta: %+v", found[0])
		}
		if found[0].Rel != filepath.Join("skills", "cat-a", "alpha") {
			t.Errorf("Rel de alpha = %q", found[0].Rel)
		}
		if found[1].Name != "beta" {
			t.Errorf("segundo item: %+v", found[1])
		}
	})

	t.Run("sem SKILL.md", func(t *testing.T) {
		if _, _, _, err := svc.Discover(t.TempDir()); err == nil {
			t.Fatal("deveria falhar sem SKILL.md")
		}
	})
}

func TestInstall(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	src1 := writeSkill(t, t.TempDir(), "nova", validMD("nova", "x"))
	src2 := writeSkill(t, t.TempDir(), "outra", validMD("outra", "y"))
	writeSkill(t, p.LibraryDir(), "nova", validMD("nova", "já existe"))

	installed, err := svc.Install([]Found{
		{SrcDir: src1, Name: "nova"},
		{SrcDir: src2, Name: "outra", Rel: "sub/outra"},
	}, Origin{Type: "git", Source: "https://github.com/x/y"})
	if err == nil {
		t.Fatal("esperava erro parcial (nova já existe)")
	}
	if len(installed) != 1 || installed[0] != "outra" {
		t.Fatalf("installed = %v", installed)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "outra", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	// origem registrada e round-trip via scan
	o := readOrigin(filepath.Join(p.LibraryDir(), "outra"))
	if o == nil || o.Type != "git" || o.Source != "https://github.com/x/y" ||
		o.Sub != "sub/outra" || o.InstalledAt.IsZero() {
		t.Fatalf("origem = %+v", o)
	}
	sk := scanOne(t, svc, nil, "outra")
	if sk.Origin == nil || sk.Origin.Source != "https://github.com/x/y" {
		t.Fatalf("scan não expôs a origem: %+v", sk.Origin)
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
		"skills/uma/SKILL.md":  validMD("uma", "do zip"),
		"skills/uma/extra.txt": "conteúdo",
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
		t.Fatal("zip deveria ter dir temporário de cleanup")
	}
	if len(found) != 1 || found[0].Name != "uma" {
		t.Fatalf("found = %+v", found)
	}
	if _, err := os.Stat(filepath.Join(cleanup, "..", "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("zip-slip não foi bloqueado")
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "uma", "extra.txt")); err != nil {
		t.Fatalf("arquivo extra não instalado: %v", err)
	}
}

// Repo com skill e hook de plugin: os dois aparecem na descoberta, e o hook
// vai para a biblioteca de hooks, não para a de skills.
func TestDiscoverAndInstallHooksFromRepo(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "code-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: code-review\ndescription: revisa diffs\n---\ninstruções.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	hookDir := filepath.Join(root, "plugins", "guarda", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"description":"guarda","hooks":{"Stop":[{"hooks":[{"type":"command","command":"bash \"${CLAUDE_PLUGIN_ROOT}/hooks/s.sh\""}]}]}}`
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
		t.Errorf("pasta local não deveria gerar temporário: %q", cleanup)
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
		t.Fatalf("descoberta = %d skills, %d hooks: %+v", skills, hookItems, found)
	}

	names, err := svc.Install(found, origin)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("instalados = %v", names)
	}
	// A skill foi para a biblioteca de skills; o hook, para a de hooks.
	if _, err := os.Stat(filepath.Join(paths.LibraryDir(), "code-review", "SKILL.md")); err != nil {
		t.Error("skill não foi instalada")
	}
	entry, err := os.ReadFile(filepath.Join(paths.HooksDir(), "guarda.json"))
	if err != nil {
		t.Fatalf("hook não foi importado: %v", err)
	}
	if strings.Contains(string(entry), "CLAUDE_PLUGIN_ROOT") {
		t.Errorf("comando não foi reescrito: %s", entry)
	}
	if _, err := os.Stat(filepath.Join(paths.HooksDir(), "guarda", "s.sh")); err != nil {
		t.Error("script do hook não foi copiado")
	}
	if _, err := os.Stat(filepath.Join(paths.LibraryDir(), "guarda")); err == nil {
		t.Error("hook não pode virar skill na biblioteca")
	}
}
