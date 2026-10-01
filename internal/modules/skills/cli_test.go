package skills

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func testSkillSvc(t *testing.T) (*Service, []agent.Agent) {
	t.Helper()
	home := t.TempDir()
	svc := New(core.PathsIn(home))
	managed := filepath.Join(home, ".claude", "skills")
	ag := agent.Agent{
		ID: "claude-code", Name: "claude-code", Short: "c",
		Installed:  true,
		ManagedDir: managed,
		ReadDirs:   []string{managed},
	}
	return svc, []agent.Agent{ag}
}

func writeSkillLib(t *testing.T, svc *Service, name, desc string) {
	t.Helper()
	dir := filepath.Join(svc.Paths().LibraryDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: " + desc + "\n---\ninstructions\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run builds this module's command registry like internal/app does and
// dispatches args.
func run(t *testing.T, args []string, svc *Service, agents []agent.Agent) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := cli.Context{Out: &out, Err: &errOut, Paths: svc.Paths(), Agents: func() []agent.Agent { return agents }}
	cmds := append(commands(svc), cli.DoctorCommand(checks(svc)))
	code = cli.Run(args, c, cmds)
	return out.String(), errOut.String(), code
}

func TestCLIUnknownCommand(t *testing.T) {
	svc, agents := testSkillSvc(t)
	_, _, code := run(t, []string{"xyzzy"}, svc, agents)
	if code != 1 {
		t.Errorf("invalid command should return 1, got %d", code)
	}
}

func TestCLINoArgs(t *testing.T) {
	svc, agents := testSkillSvc(t)
	_, _, code := run(t, []string{}, svc, agents)
	if code != 1 {
		t.Errorf("no args should return 1, got %d", code)
	}
}

func TestCLIList(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "my-skill", "does things")

	stdout, stderr, code := run(t, []string{"list"}, svc, agents)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "my-skill") {
		t.Errorf("list: %q lacks my-skill", stdout)
	}
}

func TestCLIListJSON(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "my-skill", "does things")

	stdout, _, code := run(t, []string{"list", "--json"}, svc, agents)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var items []jsonSkillItem
	if err := json.Unmarshal([]byte(stdout), &items); err != nil {
		t.Fatalf("invalid JSON: %v — output: %q", err, stdout)
	}
	if len(items) != 1 || items[0].Dir != "my-skill" {
		t.Errorf("list --json: %+v", items)
	}
}

func TestCLIEnableDisable(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "sk", "desc")

	stdout, stderr, code := run(t, []string{"enable", "sk"}, svc, agents)
	if code != 0 {
		t.Fatalf("enable exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "enabled") {
		t.Errorf("enable output: %q", stdout)
	}
	link := filepath.Join(agents[0].ManagedDir, "sk")
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink not created: %v", err)
	}

	stdout, stderr, code = run(t, []string{"disable", "sk"}, svc, agents)
	if code != 0 {
		t.Fatalf("disable exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "disabled") {
		t.Errorf("disable output: %q", stdout)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("symlink should be gone")
	}
}

func TestCLIEnableSpecificAgent(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "sk", "desc")

	_, stderr, code := run(t, []string{"enable", "sk", "--agent", "claude-code"}, svc, agents)
	if code != 0 {
		t.Fatalf("enable --agent exit %d: %s", code, stderr)
	}
	_, _, code = run(t, []string{"enable", "sk", "--agent", "missing"}, svc, agents)
	if code != 1 {
		t.Errorf("enable with an invalid agent should fail")
	}
}

func TestCLIEnableMissingSkill(t *testing.T) {
	svc, agents := testSkillSvc(t)
	_, _, code := run(t, []string{"enable", "missing"}, svc, agents)
	if code != 1 {
		t.Errorf("enable of a missing skill should fail")
	}
}

func TestCLIInstall(t *testing.T) {
	svc, agents := testSkillSvc(t)
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "SKILL.md"), 0o755); err == nil {
		// SKILL.md is a file, not a dir: recreate
	}
	os.Remove(filepath.Join(srcDir, "SKILL.md"))
	md := "---\nname: from-dir\ndescription: test\n---\n"
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(md), 0o644)

	stdout, stderr, code := run(t, []string{"install", srcDir}, svc, agents)
	if code != 0 {
		t.Fatalf("install exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "from-dir") {
		t.Errorf("install output: %q", stdout)
	}
	_, _, code = run(t, []string{"install"}, svc, agents)
	if code != 1 {
		t.Errorf("install without args should fail")
	}
}

func TestCLIRemove(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "sk", "desc")

	stdout, stderr, code := run(t, []string{"remove", "sk"}, svc, agents)
	if code != 0 {
		t.Fatalf("remove exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("remove output: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(svc.Paths().LibraryDir(), "sk")); !os.IsNotExist(err) {
		t.Errorf("skill should be gone from the library")
	}
	_, _, code = run(t, []string{"remove", "missing"}, svc, agents)
	if code != 1 {
		t.Errorf("remove of a missing skill should fail")
	}
}

func TestCLIAdoptMissingFlags(t *testing.T) {
	svc, agents := testSkillSvc(t)
	_, _, code := run(t, []string{"adopt", "sk"}, svc, agents)
	if code != 1 {
		t.Errorf("adopt without --agent should fail")
	}
	_, _, code = run(t, []string{"adopt", "--agent", "claude-code"}, svc, agents)
	if code != 1 {
		t.Errorf("adopt without a name should fail")
	}
}

func TestCLIDoctor(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "ok-skill", "desc")

	stdout, stderr, code := run(t, []string{"doctor"}, svc, agents)
	if code != 0 {
		t.Fatalf("doctor exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "claude-code") {
		t.Errorf("doctor should list claude-code: %q", stdout)
	}
	if !strings.Contains(stdout, "all OK") {
		t.Errorf("doctor should report OK: %q", stdout)
	}
}

func TestCLIDoctorInvalidSkill(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "no-description", "")

	stdout, _, code := run(t, []string{"doctor"}, svc, agents)
	if code != 1 {
		t.Errorf("doctor with an invalid skill should return 1, got %d", code)
	}
	if !strings.Contains(stdout, "description") || !strings.Contains(stdout, "empty") {
		t.Errorf("doctor should report the empty description: %q", stdout)
	}
}

func TestCLIDoctorBrokenSymlink(t *testing.T) {
	svc, agents := testSkillSvc(t)
	// broken symlink in the agent dir
	managed := agents[0].ManagedDir
	os.MkdirAll(managed, 0o755)
	os.Symlink("/nonexistent/path/skill", filepath.Join(managed, "broken"))

	stdout, _, _ := run(t, []string{"doctor"}, svc, agents)
	if !strings.Contains(stdout, "broken symlink") {
		t.Errorf("doctor should report the broken symlink: %q", stdout)
	}
}

// `skills <sub>` is the same as the top-level command; without sub, it lists.
func TestCLISkillsGroup(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "my-skill", strings.Repeat("long description ", 20))
	for _, args := range [][]string{{"skills"}, {"skills", "list"}} {
		stdout, stderr, code := run(t, args, svc, agents)
		if code != 0 || !strings.Contains(stdout, "my-skill") {
			t.Fatalf("%v = exit %d, %q %q", args, code, stdout, stderr)
		}
		if strings.Count(stdout, "long description") > 4 || !strings.Contains(stdout, "…") {
			t.Errorf("description not truncated: %q", stdout)
		}
	}
	if _, _, code := run(t, []string{"skills", "xyz"}, svc, agents); code != 1 {
		t.Errorf("unknown subcommand = %d", code)
	}
}

// Without --agent, enable targets only installed agents with a skills dir,
// like the TUI a key: no dirs appear for agents that are not installed.
func TestCLIEnableSkipsAgentsNotInstalled(t *testing.T) {
	svc, agents := testSkillSvc(t)
	home := svc.Paths().Home
	codexDir := filepath.Join(home, ".agents", "skills")
	agents = append(agents,
		agent.Agent{ID: "codex", Name: "Codex", ManagedDir: codexDir, ReadDirs: []string{codexDir}},
		agent.Agent{ID: "claude-desktop", Name: "Claude Desktop", Installed: true},
	)
	writeSkillLib(t, svc, "sk", "desc")

	_, stderr, code := run(t, []string{"enable", "sk"}, svc, agents)
	if code != 0 {
		t.Fatalf("enable exit %d stderr=%q", code, stderr)
	}
	if _, err := os.Lstat(filepath.Join(agents[0].ManagedDir, "sk")); err != nil {
		t.Errorf("claude-code link missing: %v", err)
	}
	if _, err := os.Stat(codexDir); !os.IsNotExist(err) {
		t.Errorf("created %s for an agent that is not installed", codexDir)
	}
	if _, _, code := run(t, []string{"enable", "sk", "--agent", "claude-code", "--all"}, svc, agents); code != 1 {
		t.Errorf("--agent with --all should be a usage error, got %d", code)
	}
}

// A link in a shared dir serves several agents: DisableAll removes it once
// instead of failing on the second agent.
func TestDisableAllSharedDir(t *testing.T) {
	svc, _ := testSkillSvc(t)
	writeSkillLib(t, svc, "sk", "desc")
	shared := filepath.Join(svc.Paths().Home, ".agents", "skills")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(svc.Paths().LibraryDir(), "sk"), filepath.Join(shared, "sk")); err != nil {
		t.Fatal(err)
	}
	agents := []agent.Agent{
		{ID: "codex", Name: "Codex", Installed: true, ManagedDir: shared, ReadDirs: []string{shared}},
		{ID: "gemini-cli", Name: "Gemini CLI", Installed: true, ManagedDir: filepath.Join(svc.Paths().Home, ".gemini", "skills"), ReadDirs: []string{shared}},
	}
	skills, err := svc.Scan(agents)
	if err != nil {
		t.Fatal(err)
	}
	sk, _ := findSkill("sk", skills)
	if err := svc.DisableAll(sk, agents); err != nil {
		t.Fatalf("DisableAll: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(shared, "sk")); !os.IsNotExist(err) {
		t.Error("shared link should be gone")
	}
}

// With no installed agent that has a skills dir, enable fails instead of
// reporting a success that enabled nothing.
func TestCLIEnableNoTargets(t *testing.T) {
	svc, agents := testSkillSvc(t)
	agents[0].Installed = false
	writeSkillLib(t, svc, "sk", "desc")
	if _, stderr, code := run(t, []string{"enable", "sk"}, svc, agents); code != 1 || !strings.Contains(stderr, "no installed agent") {
		t.Errorf("exit %d stderr=%q, want an error", code, stderr)
	}
}

// writeSource makes a local source folder with one skill per name.
func writeSource(t *testing.T, names ...string) string {
	t.Helper()
	src := t.TempDir()
	for _, n := range names {
		dir := filepath.Join(src, "skills", n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: " + n + "\ndescription: test\n---\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func TestCLIInstallSubset(t *testing.T) {
	svc, agents := testSkillSvc(t)
	src := writeSource(t, "one", "two", "three")

	stdout, stderr, code := run(t, []string{"install", src, "one", "three"}, svc, agents)
	if code != 0 {
		t.Fatalf("install exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "installed: one") || !strings.Contains(stdout, "installed: three") || strings.Contains(stdout, "two") {
		t.Errorf("install output: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(svc.Paths().LibraryDir(), "two")); !os.IsNotExist(err) {
		t.Errorf("a skill not picked should not be installed")
	}
}

func TestCLIInstallUnknownSkill(t *testing.T) {
	svc, agents := testSkillSvc(t)
	src := writeSource(t, "one", "two")

	_, stderr, code := run(t, []string{"install", src, "one", "nope"}, svc, agents)
	if code != 1 {
		t.Fatalf("an unknown skill should fail, got %d", code)
	}
	if !strings.Contains(stderr, "nope") || !strings.Contains(stderr, "available: one, two") {
		t.Errorf("stderr should name the unknown skill and list the available ones: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(svc.Paths().LibraryDir(), "one")); !os.IsNotExist(err) {
		t.Errorf("nothing should be installed when a name is unknown")
	}
}

func TestCLIInstallAll(t *testing.T) {
	svc, agents := testSkillSvc(t)
	src := writeSource(t, "one", "two")

	// flags after the skill names too
	stdout, stderr, code := run(t, []string{"install", src, "one", "--all"}, svc, agents)
	if code != 0 {
		t.Fatalf("install exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, `skill "one" enabled`) {
		t.Errorf("install --all output: %q", stdout)
	}
	link := filepath.Join(agents[0].ManagedDir, "one")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("--all should link the skill into the agent: %v", err)
	}
}

func TestCLIInstallAllNoAgent(t *testing.T) {
	svc, agents := testSkillSvc(t)
	agents[0].Installed = false
	src := writeSource(t, "one")

	_, _, code := run(t, []string{"install", src, "--all"}, svc, agents)
	if code != 1 {
		t.Fatalf("--all with no installed agent should fail, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(svc.Paths().LibraryDir(), "one")); !os.IsNotExist(err) {
		t.Errorf("nothing should be installed when --all has nowhere to enable")
	}
}
