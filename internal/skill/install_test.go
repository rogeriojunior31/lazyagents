package skill

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
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
		found, cleanup, err := svc.Discover(root)
		if err != nil || cleanup != "" {
			t.Fatalf("err=%v cleanup=%q", err, cleanup)
		}
		if len(found) != 1 || found[0].Name != "solo" || !found[0].Valid {
			t.Fatalf("found = %+v", found)
		}
	})

	t.Run("aninhado com dedupe de dir oculto", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, filepath.Join(root, "skills", "cat-a"), "alpha", validMD("alpha", "a"))
		writeSkill(t, filepath.Join(root, "skills", "cat-b"), "beta", validMD("beta", "b"))
		writeSkill(t, filepath.Join(root, ".openclaw", "skills"), "alpha", validMD("alpha", "dup oculta"))
		writeSkill(t, filepath.Join(root, "node_modules", "x"), "gamma", validMD("gamma", "ignorada"))
		found, _, err := svc.Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 2 {
			t.Fatalf("esperava alpha e beta, veio %+v", found)
		}
		if found[0].Name != "alpha" || found[0].Hidden {
			t.Errorf("alpha deveria ser a versão não-oculta: %+v", found[0])
		}
		if found[1].Name != "beta" {
			t.Errorf("segundo item: %+v", found[1])
		}
	})

	t.Run("sem SKILL.md", func(t *testing.T) {
		if _, _, err := svc.Discover(t.TempDir()); err == nil {
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
		{SrcDir: src2, Name: "outra"},
	})
	if err == nil {
		t.Fatal("esperava erro parcial (nova já existe)")
	}
	if len(installed) != 1 || installed[0] != "outra" {
		t.Fatalf("installed = %v", installed)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "outra", "SKILL.md")); err != nil {
		t.Fatal(err)
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

	found, cleanup, err := svc.Discover(zipPath)
	if err != nil {
		t.Fatal(err)
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
	if _, err := svc.Install(found); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "uma", "extra.txt")); err != nil {
		t.Fatalf("arquivo extra não instalado: %v", err)
	}
}
