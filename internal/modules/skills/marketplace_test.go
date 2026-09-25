package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// marketplaceRepo builds a repo in the Claude Code marketplace layout: plugin
// "docs" with the default skills/ folder, "shared" pointing at root skills via
// "skills", an external GitHub one and one that tries to escape.
func marketplaceRepo(t *testing.T, root string) {
	t.Helper()
	writeSkill(t, filepath.Join(root, "plugins", "docs", "skills"), "readme-writer", validMD("readme-writer", "writes READMEs"))
	writeSkill(t, filepath.Join(root, "skills"), "xlsx", validMD("xlsx", "spreadsheets"))
	writeSkill(t, filepath.Join(root, "skills"), "pdf", validMD("pdf", "pdfs"))
	writeSkill(t, filepath.Join(root, "skills"), "unlisted", validMD("unlisted", "not listed by any plugin"))
	mp := `{
  "name": "acme",
  "owner": {"name": "Acme"},
  "metadata": {"pluginRoot": "./plugins"},
  "plugins": [
    {"name": "docs", "source": "docs", "description": "documentation"},
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
	if len(found) != 3 || got["unlisted"].Name != "" {
		t.Fatalf("want readme-writer, xlsx and pdf (not unlisted), got %+v", found)
	}
	if f := got["readme-writer"]; f.Plugin != "docs" || f.Rel != filepath.Join("plugins", "docs", "skills", "readme-writer") {
		t.Errorf("readme-writer = plugin %q rel %q", f.Plugin, f.Rel)
	}
	if f := got["xlsx"]; f.Plugin != "shared" || f.Rel != filepath.Join("skills", "xlsx") {
		t.Errorf("xlsx = plugin %q rel %q (the first declaration wins)", f.Plugin, f.Rel)
	}
	notes := strings.Join(origin.Notes, "\n")
	for _, want := range []string{"acme/deploy-plugin", `unsupported source "command"`, "outside the repository"} {
		if !strings.Contains(notes, want) {
			t.Errorf("missing note %q in:\n%s", want, notes)
		}
	}
}

func TestDiscoverMarketplace_Fallbacks(t *testing.T) {
	svc := New(testPaths(t))

	t.Run("invalid json", func(t *testing.T) {
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
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("marketplace without skills falls back to the scan", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, filepath.Join(root, "other"), "loose", validMD("loose", "s"))
		if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
			t.Fatal(err)
		}
		mp := `{"name":"m","plugins":[{"name":"ext","source":{"source":"url","url":"https://x/y.git"}}]}`
		if err := os.WriteFile(filepath.Join(root, marketplacePath), []byte(mp), 0o644); err != nil {
			t.Fatal(err)
		}
		found, origin, _, err := svc.Discover(root)
		if err != nil || len(found) != 1 || found[0].Name != "loose" || found[0].Plugin != "" {
			t.Fatalf("found = %+v err = %v", found, err)
		}
		if len(origin.Notes) != 1 || !strings.Contains(origin.Notes[0], "https://x/y.git") {
			t.Errorf("notes = %v", origin.Notes)
		}
	})
}

// End to end: git repo with a marketplace → shallow clone → install one entry →
// the recorded origin (Sub) points at the right path in the repo.
func TestInstallFromMarketplaceRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
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
		t.Fatalf("readme-writer not listed: %+v", found)
	}
	names, err := svc.Install(pick, origin)
	if err != nil || len(names) != 1 {
		t.Fatalf("Install = %v %v", names, err)
	}
	o := readOrigin(filepath.Join(svc.paths.LibraryDir(), "readme-writer"))
	if o == nil || o.Type != "git" || o.Sub != filepath.Join("plugins", "docs", "skills", "readme-writer") {
		t.Errorf("recorded origin = %+v", o)
	}
}
