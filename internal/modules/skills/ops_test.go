package skills

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

func testAgent(home, id, managed string, extra ...string) agent.Agent {
	m := filepath.Join(home, managed)
	dirs := []string{m}
	for _, e := range extra {
		dirs = append(dirs, filepath.Join(home, e))
	}
	return agent.Agent{ID: id, Name: id, Short: id[:1], Installed: true, ManagedDir: m, ReadDirs: dirs}
}

func scanOne(t *testing.T, svc *Service, agents []agent.Agent, dir string) Skill {
	t.Helper()
	skills, err := svc.Scan(agents)
	if err != nil {
		t.Fatal(err)
	}
	for _, sk := range skills {
		if sk.Dir == dir {
			return sk
		}
	}
	t.Fatalf("skill %s missing from the scan", dir)
	return Skill{}
}

func TestCreate(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	path, err := svc.Create("my-skill")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := ParseMeta(data)
	if !ok || meta.Name != "my-skill" || meta.Description == "" {
		t.Fatalf("invalid template: ok=%v meta=%+v", ok, meta)
	}
	sk := scanOne(t, svc, nil, "my-skill")
	if !sk.InLibrary || !sk.Valid {
		t.Fatalf("created skill: %+v", sk)
	}
	if _, err := svc.Create("my-skill"); err == nil {
		t.Fatal("duplicate name should fail")
	}
	for _, bad := range []string{"", "Uppercase", "with space", "-leading-hyphen", "trailing-", "a_b", "a/b"} {
		if _, err := svc.Create(bad); err == nil {
			t.Errorf("name %q should be invalid", bad)
		}
	}
}

func TestEnableDisable(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "d"))
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}

	sk := scanOne(t, svc, agents, "sk")
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ag.ManagedDir, "sk")
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(p.LibraryDir(), "sk") {
		t.Fatalf("wrong symlink: %q, %v", target, err)
	}

	sk = scanOne(t, svc, agents, "sk")
	if st := sk.States[ag.ID]; !st.On || !st.Managed {
		t.Fatalf("state after enable: %+v", st)
	}
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatalf("enable should be idempotent: %v", err)
	}
	if err := svc.Disable(sk, ag); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("symlink should be gone: %v", err)
	}

	// real dir: disable refuses and leaves the content
	writeSkill(t, ag.ManagedDir, "real", validMD("real", "local"))
	real := scanOne(t, svc, agents, "real")
	if err := svc.Disable(real, ag); err == nil {
		t.Fatal("disable of a real dir should fail")
	}
	if _, err := os.Stat(filepath.Join(ag.ManagedDir, "real", "SKILL.md")); err != nil {
		t.Fatalf("local content was touched: %v", err)
	}

	// a skill outside the library cannot be enabled in another agent
	if err := svc.Enable(real, testAgent(p.Home, "codex", ".agents/skills")); err == nil {
		t.Fatal("enable outside the library should fail")
	}
	if err := svc.Enable(sk, agent.Agent{ID: "desktop", Name: "desktop", Installed: true}); err == nil {
		t.Fatal("enable without ManagedDir should fail")
	}
	// name already taken by a real dir in the agent
	writeSkill(t, p.LibraryDir(), "real", validMD("real", "in the library"))
	realLib := scanOne(t, svc, agents, "real")
	realLib.InLibrary = true
	realLib.States = map[string]AgentState{} // forces the Lstat path
	if err := svc.Enable(realLib, ag); err == nil {
		t.Fatal("enable over an existing real dir should fail")
	}
}

func TestEnableAllDisableAll(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "d"))
	a1 := testAgent(p.Home, "claude-code", ".claude/skills")
	a2 := testAgent(p.Home, "codex", ".agents/skills")
	agents := []agent.Agent{a1, a2, {ID: "no-skills", Name: "x", Installed: true}}

	sk := scanOne(t, svc, agents, "sk")
	if err := svc.EnableAll(sk, agents); err != nil {
		t.Fatal(err)
	}
	for _, ag := range []agent.Agent{a1, a2} {
		if _, err := os.Lstat(filepath.Join(ag.ManagedDir, "sk")); err != nil {
			t.Errorf("missing link in %s: %v", ag.ID, err)
		}
	}

	// DisableAll leaves a local skill in another agent alone
	a3 := testAgent(p.Home, "gemini-cli", ".gemini/skills")
	writeSkill(t, a3.ManagedDir, "sk", validMD("sk", "local"))
	all := []agent.Agent{a1, a2, a3}
	sk = scanOne(t, svc, all, "sk")
	if err := svc.DisableAll(sk, all); err != nil {
		t.Fatal(err)
	}
	for _, ag := range []agent.Agent{a1, a2} {
		if _, err := os.Lstat(filepath.Join(ag.ManagedDir, "sk")); !os.IsNotExist(err) {
			t.Errorf("link left in %s", ag.ID)
		}
	}
	if _, err := os.Stat(filepath.Join(a3.ManagedDir, "sk", "SKILL.md")); err != nil {
		t.Errorf("local skill should not be touched: %v", err)
	}
}

func TestAdopt(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}
	writeSkill(t, ag.ManagedDir, "mine", validMD("mine", "local"))

	sk := scanOne(t, svc, agents, "mine")
	if err := svc.Adopt(sk, ag); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "mine", "SKILL.md")); err != nil {
		t.Fatalf("not copied into the library: %v", err)
	}
	link := filepath.Join(ag.ManagedDir, "mine")
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(p.LibraryDir(), "mine") {
		t.Fatalf("source did not become a symlink: %q, %v", target, err)
	}
	if backups, _ := os.ReadDir(p.BackupsDir()); len(backups) != 1 {
		t.Fatalf("want 1 backup, have %d", len(backups))
	}
	if o := readOrigin(filepath.Join(p.LibraryDir(), "mine")); o == nil || o.Type != "dir" {
		t.Fatalf("adopt should record a dir origin: %+v", o)
	}
	// second adoption: already in the library
	sk = scanOne(t, svc, agents, "mine")
	if err := svc.Adopt(sk, ag); err == nil {
		t.Fatal("re-adopt should fail")
	}
	// a third-party symlink source is refused
	foreign := writeSkill(t, t.TempDir(), "foreign", validMD("foreign", "x"))
	mustSymlink(t, foreign, filepath.Join(ag.ManagedDir, "foreign"))
	al := scanOne(t, svc, agents, "foreign")
	if err := svc.Adopt(al, ag); err == nil {
		t.Fatal("adopting a foreign symlink should fail")
	}
}

func TestAdoptAll(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}
	writeSkill(t, ag.ManagedDir, "local-one", validMD("local-one", "d1"))
	writeSkill(t, ag.ManagedDir, "local-two", validMD("local-two", "d2"))
	writeSkill(t, p.LibraryDir(), "in-lib", validMD("in-lib", "d3"))

	skills, err := svc.Scan(agents)
	if err != nil {
		t.Fatal(err)
	}
	adopted, errs := svc.AdoptAll(skills, agents)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(adopted) != 2 {
		t.Fatalf("want 2 adopted, got %v", adopted)
	}
	for _, dir := range []string{"local-one", "local-two"} {
		if _, err := os.Stat(filepath.Join(p.LibraryDir(), dir, "SKILL.md")); err != nil {
			t.Errorf("%s not in the library: %v", dir, err)
		}
		link := filepath.Join(ag.ManagedDir, dir)
		if target, err := os.Readlink(link); err != nil || target != filepath.Join(p.LibraryDir(), dir) {
			t.Errorf("%s: wrong symlink back: %q, %v", dir, target, err)
		}
	}

	// rerun on the same scan (already adopted): no locals left, idempotent
	skills, err = svc.Scan(agents)
	if err != nil {
		t.Fatal(err)
	}
	adopted, errs = svc.AdoptAll(skills, agents)
	if len(adopted) != 0 || len(errs) != 0 {
		t.Fatalf("no locals left should be a no-op: adopted=%v errs=%v", adopted, errs)
	}

	// one failure does not stop the rest: a foreign symlink next to a valid local
	foreign := writeSkill(t, t.TempDir(), "foreign", validMD("foreign", "x"))
	mustSymlink(t, foreign, filepath.Join(ag.ManagedDir, "foreign"))
	writeSkill(t, ag.ManagedDir, "local-three", validMD("local-three", "d4"))
	skills, err = svc.Scan(agents)
	if err != nil {
		t.Fatal(err)
	}
	adopted, errs = svc.AdoptAll(skills, agents)
	if len(errs) != 1 {
		t.Fatalf("want 1 error (foreign symlink), got %v", errs)
	}
	if len(adopted) != 1 || adopted[0] != "local-three" {
		t.Fatalf("want only local-three adopted, got %v", adopted)
	}
}

func TestRemove(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "d"))
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	other := testAgent(p.Home, "codex", ".agents/skills")
	agents := []agent.Agent{ag, other}

	sk := scanOne(t, svc, agents, "sk")
	if err := svc.EnableAll(sk, agents); err != nil {
		t.Fatal(err)
	}
	// a third-party symlink with the same name must not be removed
	foreign := writeSkill(t, t.TempDir(), "sk", validMD("sk", "from elsewhere"))
	foreignLink := filepath.Join(p.Home, ".gemini", "skills", "sk")
	mustSymlink(t, foreign, foreignLink)
	third := testAgent(p.Home, "gemini-cli", ".gemini/skills")
	all := []agent.Agent{ag, other, third}

	sk = scanOne(t, svc, all, "sk")
	if err := svc.Remove(sk, all); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk")); !os.IsNotExist(err) {
		t.Fatal("skill should leave the library")
	}
	for _, a := range []agent.Agent{ag, other} {
		if _, err := os.Lstat(filepath.Join(a.ManagedDir, "sk")); !os.IsNotExist(err) {
			t.Errorf("managed link left in %s", a.ID)
		}
	}
	if _, err := os.Lstat(foreignLink); err != nil {
		t.Error("third-party symlink should not be touched")
	}
	if backups, _ := os.ReadDir(p.BackupsDir()); len(backups) != 1 {
		t.Fatalf("want a remove backup, have %d", len(backups))
	}
	// removing a skill outside the library
	writeSkill(t, ag.ManagedDir, "local", validMD("local", "x"))
	loc := scanOne(t, svc, all, "local")
	if err := svc.Remove(loc, all); err == nil {
		t.Fatal("remove outside the library should fail")
	}
}

func TestUpdate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	p := testPaths(t)
	svc := New(p)
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}

	repoDir := t.TempDir()
	gitExec := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	gitExec("init")
	gitExec("config", "user.email", "test@test")
	gitExec("config", "user.name", "test")
	writeSkill(t, repoDir, "my-skill", validMD("my-skill", "version 1"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	// install via a file:// URL (cloneShallow accepts local URLs)
	found, origin, cleanup, err := svc.Discover("file://" + repoDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	names, err := svc.Install(found, origin)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(names) != 1 || names[0] != "my-skill" {
		t.Fatalf("Install names = %v", names)
	}

	// enable in the agent to check the symlink survives the update
	sk := scanOne(t, svc, agents, "my-skill")
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(ag.ManagedDir, "my-skill")

	// new skill version in the repo
	if err := os.WriteFile(filepath.Join(repoDir, "my-skill", "SKILL.md"),
		[]byte(validMD("my-skill", "version 2")), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec("add", ".")
	gitExec("commit", "-m", "v2")

	sk = scanOne(t, svc, agents, "my-skill")
	if err := svc.Update(sk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(p.LibraryDir(), "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading SKILL.md after update: %v", err)
	}
	if !strings.Contains(string(data), "version 2") {
		t.Errorf("content not updated: %s", data)
	}

	backups, _ := os.ReadDir(p.BackupsDir())
	if len(backups) != 1 {
		t.Fatalf("want 1 backup after update, have %d", len(backups))
	}

	// the activation symlink survived
	if _, err := os.Lstat(linkPath); err != nil {
		t.Errorf("symlink gone after update: %v", err)
	}

	// .origin.json kept, InstalledAt refreshed
	o := readOrigin(filepath.Join(p.LibraryDir(), "my-skill"))
	if o == nil || o.Type != "git" {
		t.Fatalf("origin lost after update: %+v", o)
	}
	if o.InstalledAt.IsZero() {
		t.Error("InstalledAt not refreshed")
	}

	// a skill without a git origin returns ErrNoGitOrigin
	writeSkill(t, p.LibraryDir(), "manual", validMD("manual", "no origin"))
	manual := scanOne(t, svc, agents, "manual")
	if err := svc.Update(manual); !errors.Is(err, ErrNoGitOrigin) {
		t.Errorf("update without origin: want ErrNoGitOrigin, got %v", err)
	}
}
