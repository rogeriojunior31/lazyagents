package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// marketplaceRepo monta um repo no layout de marketplace do Claude Code:
// plugin "docs" com a pasta padrão skills/, plugin "shared" que aponta para
// skills da raiz via "skills", um externo no GitHub e um que tenta escapar.
func marketplaceRepo(t *testing.T, root string) {
	t.Helper()
	writeSkill(t, filepath.Join(root, "plugins", "docs", "skills"), "readme-writer", validMD("readme-writer", "escreve READMEs"))
	writeSkill(t, filepath.Join(root, "skills"), "xlsx", validMD("xlsx", "planilhas"))
	writeSkill(t, filepath.Join(root, "skills"), "pdf", validMD("pdf", "pdfs"))
	writeSkill(t, filepath.Join(root, "skills"), "fora", validMD("fora", "não listada por nenhum plugin"))
	mp := `{
  "name": "acme",
  "owner": {"name": "Acme"},
  "metadata": {"pluginRoot": "./plugins"},
  "plugins": [
    {"name": "docs", "source": "docs", "description": "documentação"},
    {"name": "shared", "source": "./", "strict": false, "skills": ["./skills/xlsx", "./skills/pdf"]},
    {"name": "again", "source": "./", "skills": "./skills/xlsx"},
    {"name": "deploy", "source": {"source": "github", "repo": "acme/deploy-plugin"}},
    {"name": "tool", "source": {"source": "command", "command": "rm -rf ~"}},
    {"name": "escape", "source": "../../etc"}
  ]
}`
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, marketplacePath), []byte(mp), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverMarketplace(t *testing.T) {
	root := t.TempDir()
	marketplaceRepo(t, root)
	found, origin, _, err := New(testPaths(t)).Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Found{}
	for _, f := range found {
		got[f.Name] = f
	}
	if len(found) != 3 || got["fora"].Name != "" {
		t.Fatalf("esperava readme-writer, xlsx e pdf (sem fora), veio %+v", found)
	}
	if f := got["readme-writer"]; f.Plugin != "docs" || f.Rel != filepath.Join("plugins", "docs", "skills", "readme-writer") {
		t.Errorf("readme-writer = plugin %q rel %q", f.Plugin, f.Rel)
	}
	if f := got["xlsx"]; f.Plugin != "shared" || f.Rel != filepath.Join("skills", "xlsx") {
		t.Errorf("xlsx = plugin %q rel %q (a primeira declaração vence)", f.Plugin, f.Rel)
	}
	notes := strings.Join(origin.Notes, "\n")
	for _, want := range []string{"acme/deploy-plugin", `unsupported source "command"`, "outside the repository"} {
		if !strings.Contains(notes, want) {
			t.Errorf("faltou aviso %q em:\n%s", want, notes)
		}
	}
}

func TestDiscoverMarketplace_Fallbacks(t *testing.T) {
	svc := New(testPaths(t))

	t.Run("json inválido", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, root, "x", validMD("x", "x"))
		if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, marketplacePath), []byte(`{"plugins": [`), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := svc.Discover(root)
		if err == nil || !strings.Contains(err.Error(), "invalid .claude-plugin/marketplace.json") {
			t.Fatalf("erro = %v", err)
		}
	})

	t.Run("marketplace sem skill cai na varredura", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, filepath.Join(root, "outra"), "solta", validMD("solta", "s"))
		if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
			t.Fatal(err)
		}
		mp := `{"name":"m","plugins":[{"name":"ext","source":{"source":"url","url":"https://x/y.git"}}]}`
		if err := os.WriteFile(filepath.Join(root, marketplacePath), []byte(mp), 0o644); err != nil {
			t.Fatal(err)
		}
		found, origin, _, err := svc.Discover(root)
		if err != nil || len(found) != 1 || found[0].Name != "solta" || found[0].Plugin != "" {
			t.Fatalf("found = %+v err = %v", found, err)
		}
		if len(origin.Notes) != 1 || !strings.Contains(origin.Notes[0], "https://x/y.git") {
			t.Errorf("notes = %v", origin.Notes)
		}
	})
}

// Ponta a ponta: repo git com marketplace → clone raso → instala uma entry →
// a origem registrada (Sub) aponta para o caminho certo dentro do repo.
func TestInstallFromMarketplaceRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não disponível")
	}
	repo := t.TempDir()
	run := makeGitRepo(t, repo)
	marketplaceRepo(t, repo)
	run("add", ".")
	run("commit", "-m", "init")

	svc := New(testPaths(t))
	found, origin, cleanup, err := svc.Discover("file://" + repo)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cleanup)
	var pick []Found
	for _, f := range found {
		if f.Name == "readme-writer" {
			pick = append(pick, f)
		}
	}
	if len(pick) != 1 {
		t.Fatalf("readme-writer não listada: %+v", found)
	}
	names, err := svc.Install(pick, origin)
	if err != nil || len(names) != 1 {
		t.Fatalf("Install = %v %v", names, err)
	}
	o := readOrigin(filepath.Join(svc.paths.LibraryDir(), "readme-writer"))
	if o == nil || o.Type != "git" || o.Sub != filepath.Join("plugins", "docs", "skills", "readme-writer") {
		t.Errorf("origem registrada = %+v", o)
	}
}
