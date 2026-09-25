package skills

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeGitRepo initializes an isolated git repo in repoDir with a first commit.
func makeGitRepo(t *testing.T, repoDir string) func(...string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+t.TempDir(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	return run
}

// installGitSkill installs a skill from a local repo (file://).
func installGitSkill(t *testing.T, svc *Service, repoURL, skillDir string, agents []interface{}) {
	t.Helper()
	found, origin, cleanup, err := svc.Discover(repoURL)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	names, err := svc.Install(found, origin)
	if err != nil {
		t.Fatalf("Install(%s): %v", skillDir, err)
	}
	found2 := false
	for _, n := range names {
		if n == skillDir {
			found2 = true
		}
	}
	if !found2 {
		t.Fatalf("Install did not return %s; got %v", skillDir, names)
	}
}

func TestCheckUpdates_UpToDate(t *testing.T) {
	repoDir := t.TempDir()
	gitExec := makeGitRepo(t, repoDir)

	writeSkill(t, repoDir, "sk-a", validMD("sk-a", "desc"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	p := testPaths(t)
	svc := New(p)

	found, origin, cleanup, err := svc.Discover("file://" + repoDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatalf("Install: %v", err)
	}

	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}

	checks, err := svc.CheckUpdates(skills)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 {
		t.Fatalf("want 1 check, got %d", len(checks))
	}
	if checks[0].Status != UpdateStatusUpToDate {
		t.Errorf("want UpToDate, got %v (err=%v)", checks[0].Status, checks[0].Err)
	}
}

func TestCheckUpdates_Available(t *testing.T) {
	repoDir := t.TempDir()
	gitExec := makeGitRepo(t, repoDir)

	writeSkill(t, repoDir, "sk-b", validMD("sk-b", "v1"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	p := testPaths(t)
	svc := New(p)

	found, origin, cleanup, err := svc.Discover("file://" + repoDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// new version on the remote
	if err := os.WriteFile(filepath.Join(repoDir, "sk-b", "SKILL.md"),
		[]byte(validMD("sk-b", "v2")), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec("add", ".")
	gitExec("commit", "-m", "v2")

	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}

	checks, err := svc.CheckUpdates(skills)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 {
		t.Fatalf("want 1 check, got %d", len(checks))
	}
	if checks[0].Status != UpdateStatusAvailable {
		t.Errorf("want Available, got %v", checks[0].Status)
	}

	updated, skipped, errs := svc.UpdateAll(checks)
	if updated != 1 || len(skipped) != 0 || len(errs) != 0 {
		t.Errorf("UpdateAll: updated=%d skipped=%v errs=%v", updated, skipped, errs)
	}
}

func TestCheckUpdates_LocallyEdited(t *testing.T) {
	repoDir := t.TempDir()
	gitExec := makeGitRepo(t, repoDir)

	writeSkill(t, repoDir, "sk-c", validMD("sk-c", "desc"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	p := testPaths(t)
	svc := New(p)

	found, origin, cleanup, err := svc.Discover("file://" + repoDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// edit the local file without Service.Update
	libPath := filepath.Join(p.LibraryDir(), "sk-c", "SKILL.md")
	if err := os.WriteFile(libPath, []byte(validMD("sk-c", "edited locally")), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}

	checks, err := svc.CheckUpdates(skills)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 {
		t.Fatalf("want 1 check, got %d", len(checks))
	}
	if checks[0].Status != UpdateStatusLocallyEdited {
		t.Errorf("want LocallyEdited, got %v", checks[0].Status)
	}

	updated, skipped, errs := svc.UpdateAll(checks)
	if updated != 0 || len(skipped) != 1 || len(errs) != 0 {
		t.Errorf("UpdateAll: updated=%d skipped=%v errs=%v", updated, skipped, errs)
	}
}

func TestCheckUpdates_NoHash(t *testing.T) {
	repoDir := t.TempDir()
	gitExec := makeGitRepo(t, repoDir)

	writeSkill(t, repoDir, "sk-d", validMD("sk-d", "desc"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	p := testPaths(t)
	svc := New(p)

	found, origin, cleanup, err := svc.Discover("file://" + repoDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// clear the hash in .origin.json, like an old install without one
	originFile := filepath.Join(p.LibraryDir(), "sk-d", ".origin.json")
	var o Origin
	data, err := os.ReadFile(originFile)
	if err != nil {
		t.Fatalf("reading .origin.json: %v", err)
	}
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatalf("unmarshal .origin.json: %v", err)
	}
	o.Hash = ""
	out, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(originFile, out, 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := svc.CheckUpdates(skills)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 {
		t.Fatalf("want 1 check, got %d", len(checks))
	}
	// no saved hash: same content → UpToDate
	if checks[0].Status == UpdateStatusUnknown && checks[0].Err != nil {
		t.Errorf("missing hash caused an unexpected error: %v", checks[0].Err)
	}
}

func TestCheckUpdates_TwoSkillsSameRepo(t *testing.T) {
	repoDir := t.TempDir()
	gitExec := makeGitRepo(t, repoDir)

	writeSkill(t, repoDir, "sk-e1", validMD("sk-e1", "desc"))
	writeSkill(t, repoDir, "sk-e2", validMD("sk-e2", "desc"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	p := testPaths(t)
	svc := New(p)

	found, origin, cleanup, err := svc.Discover("file://" + repoDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	if _, err := svc.Install(found, origin); err != nil {
		t.Fatalf("Install: %v", err)
	}

	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) < 2 {
		t.Fatalf("want >=2 skills, got %d", len(skills))
	}

	checks, err := svc.CheckUpdates(skills)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 2 {
		t.Fatalf("want 2 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if c.Err != nil {
			t.Errorf("skill %s: unexpected error: %v", c.Skill.Dir, c.Err)
		}
		if c.Status != UpdateStatusUpToDate {
			t.Errorf("skill %s: want UpToDate, got %v", c.Skill.Dir, c.Status)
		}
	}
}
