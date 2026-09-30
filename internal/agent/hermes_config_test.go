package agent

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// external_dirs and create_dir resolve like Hermes does: ${VAR} and ~
// expanded, relative to the Hermes home, missing dirs and duplicates dropped;
// ~/.agents/skills makes Hermes a reader of the shared dir.
func TestHermesSkillDirs(t *testing.T) {
	home := t.TempDir()
	hermes := filepath.Join(home, ".hermes")
	team, repo, created := filepath.Join(home, "team"), filepath.Join(home, "repo", "skills"), filepath.Join(hermes, "made")
	shared := filepath.Join(home, ".agents", "skills")
	mkdirs(t, filepath.Join(hermes, "skills"), team, repo, created, shared)
	t.Setenv("SKILLS_REPO", filepath.Join(home, "repo"))
	writeFile(t, filepath.Join(hermes, "config.yaml"), `skills:
  create_dir: made
  external_dirs:
    - ~/team
    - ${SKILLS_REPO}/skills
    - ~/.agents/skills
    - ~/team/
    - ~/missing
    - skills
`)
	a := (&Hermes{Home: home, Look: noBin}).Detect()
	want := []string{filepath.Join(hermes, "skills"), realPath(t, created), realPath(t, team), realPath(t, repo), shared}
	if !a.Installed || !slices.Equal(a.ReadDirs, want) {
		t.Fatalf("ReadDirs =\n%v\nwant\n%v", a.ReadDirs, want)
	}
	if a.ManagedDir != filepath.Join(hermes, "skills") || a.SharedDir != shared {
		t.Errorf("ManagedDir = %s, SharedDir = %s", a.ManagedDir, a.SharedDir)
	}

	// one string instead of a list is accepted too
	writeFile(t, filepath.Join(hermes, "config.yaml"), "skills:\n  external_dirs: ~/team\n")
	if a := (&Hermes{Home: home, Look: noBin}).Detect(); len(a.ReadDirs) != 2 || a.ReadDirs[1] != realPath(t, team) || a.SharedDir != "" {
		t.Errorf("scalar external_dirs: %v (shared %q)", a.ReadDirs, a.SharedDir)
	}
	// a config Hermes cannot parse adds nothing
	writeFile(t, filepath.Join(hermes, "config.yaml"), "skills: [unclosed\n")
	if a := (&Hermes{Home: home, Look: noBin}).Detect(); len(a.ReadDirs) != 1 {
		t.Errorf("broken config: %v", a.ReadDirs)
	}
}

// With no HERMES_HOME, `hermes profile use` picks ~/.hermes/profiles/<name>;
// HERMES_HOME wins and expands ~ and $VAR.
func TestHermesHome(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".hermes")
	coder := filepath.Join(root, "profiles", "coder")
	mkdirs(t, coder)
	writeFile(t, filepath.Join(root, "active_profile"), "coder\n")
	if got := (&Hermes{Home: home}).configDir(); got != coder {
		t.Errorf("active profile: %s, want %s", got, coder)
	}
	for _, bad := range []string{"default", "../escape", "ghost"} {
		writeFile(t, filepath.Join(root, "active_profile"), bad)
		if got := (&Hermes{Home: home}).configDir(); got != root {
			t.Errorf("active_profile %q: %s, want %s", bad, got, root)
		}
	}
	t.Setenv("HERMES_DATA", filepath.Join(home, "data"))
	if got := (&Hermes{Home: home, HermesHome: "$HERMES_DATA/h"}).configDir(); got != filepath.Join(home, "data", "h") {
		t.Errorf("HERMES_HOME with $VAR: %s", got)
	}
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// On Windows Hermes lives in %LOCALAPPDATA%\hermes, profiles included.
func TestHermesWindowsHome(t *testing.T) {
	home, appdata := t.TempDir(), t.TempDir()
	root := filepath.Join(appdata, "hermes")
	mkdirs(t, filepath.Join(root, "skills"))
	h := &Hermes{Home: home, LocalAppData: appdata, Look: noBin}
	if a := h.Detect(); !a.Installed || a.ManagedDir != filepath.Join(root, "skills") {
		t.Errorf("Detect = %+v", a)
	}
	mkdirs(t, filepath.Join(root, "profiles", "work"))
	writeFile(t, filepath.Join(root, "active_profile"), "work")
	if got := h.configDir(); got != filepath.Join(root, "profiles", "work") {
		t.Errorf("profile on Windows: %s", got)
	}
}
