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
	md := "---\nname: " + name + "\ndescription: " + desc + "\n---\ninstruções da \n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run monta o registro de comandos deste módulo como internal/app faz e
// despacha args.
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
		t.Errorf("comando inválido deveria retornar 1, got %d", code)
	}
}

func TestCLINoArgs(t *testing.T) {
	svc, agents := testSkillSvc(t)
	_, _, code := run(t, []string{}, svc, agents)
	if code != 1 {
		t.Errorf("sem args deveria retornar 1, got %d", code)
	}
}

func TestCLIList(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "my-skill", "faz coisas")

	stdout, stderr, code := run(t, []string{"list"}, svc, agents)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "my-skill") {
		t.Errorf("list: %q não contém my-skill", stdout)
	}
}

func TestCLIListJSON(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "my-skill", "faz coisas")

	stdout, _, code := run(t, []string{"list", "--json"}, svc, agents)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var items []jsonSkillItem
	if err := json.Unmarshal([]byte(stdout), &items); err != nil {
		t.Fatalf("JSON inválido: %v — output: %q", err, stdout)
	}
	if len(items) != 1 || items[0].Dir != "my-skill" {
		t.Errorf("list --json: %+v", items)
	}
}

func TestCLIEnableDisable(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "sk", "desc")

	// enable
	stdout, stderr, code := run(t, []string{"enable", "sk"}, svc, agents)
	if code != 0 {
		t.Fatalf("enable exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "enabled") {
		t.Errorf("enable output: %q", stdout)
	}
	link := filepath.Join(agents[0].ManagedDir, "sk")
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink não criado: %v", err)
	}

	// disable
	stdout, stderr, code = run(t, []string{"disable", "sk"}, svc, agents)
	if code != 0 {
		t.Fatalf("disable exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "disabled") {
		t.Errorf("disable output: %q", stdout)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("symlink deveria ter sumido")
	}
}

func TestCLIEnableSpecificAgent(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "sk", "desc")

	_, stderr, code := run(t, []string{"enable", "sk", "--agent", "claude-code"}, svc, agents)
	if code != 0 {
		t.Fatalf("enable --agent exit %d: %s", code, stderr)
	}
	// agente inválido
	_, _, code = run(t, []string{"enable", "sk", "--agent", "nao-existe"}, svc, agents)
	if code != 1 {
		t.Errorf("enable agente inválido deveria falhar")
	}
}

func TestCLIEnableMissingSkill(t *testing.T) {
	svc, agents := testSkillSvc(t)
	_, _, code := run(t, []string{"enable", "nao-existe"}, svc, agents)
	if code != 1 {
		t.Errorf("enable skill inexistente deveria falhar")
	}
}

func TestCLIInstall(t *testing.T) {
	svc, agents := testSkillSvc(t)
	// cria pasta local com uma skill
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "SKILL.md"), 0o755); err == nil {
		// SKILL.md é arquivo, não dir — recria
	}
	os.Remove(filepath.Join(srcDir, "SKILL.md"))
	md := "---\nname: from-dir\ndescription: teste\n---\n"
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(md), 0o644)

	stdout, stderr, code := run(t, []string{"install", srcDir}, svc, agents)
	if code != 0 {
		t.Fatalf("install exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "from-dir") {
		t.Errorf("install output: %q", stdout)
	}
	// install sem args → erro
	_, _, code = run(t, []string{"install"}, svc, agents)
	if code != 1 {
		t.Errorf("install sem args deveria falhar")
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
		t.Errorf("skill deveria ter sido removida da biblioteca")
	}
	// remove skill inexistente
	_, _, code = run(t, []string{"remove", "nao-existe"}, svc, agents)
	if code != 1 {
		t.Errorf("remove skill inexistente deveria falhar")
	}
}

func TestCLIAdoptMissingFlags(t *testing.T) {
	svc, agents := testSkillSvc(t)
	// sem --agent
	_, _, code := run(t, []string{"adopt", "sk"}, svc, agents)
	if code != 1 {
		t.Errorf("adopt sem --agent deveria falhar")
	}
	// sem nome da skill
	_, _, code = run(t, []string{"adopt", "--agent", "claude-code"}, svc, agents)
	if code != 1 {
		t.Errorf("adopt sem nome deveria falhar")
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
		t.Errorf("doctor deveria listar claude-code: %q", stdout)
	}
	if !strings.Contains(stdout, "all OK") {
		t.Errorf("doctor deveria reportar OK: %q", stdout)
	}
}

func TestCLIDoctorInvalidSkill(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "sem-descricao", "")

	stdout, _, code := run(t, []string{"doctor"}, svc, agents)
	if code != 1 {
		t.Errorf("doctor com skill inválida deveria retornar 1, got %d", code)
	}
	if !strings.Contains(stdout, "description") || !strings.Contains(stdout, "empty") {
		t.Errorf("doctor deveria reportar description vazia: %q", stdout)
	}
}

func TestCLIDoctorBrokenSymlink(t *testing.T) {
	svc, agents := testSkillSvc(t)
	// cria symlink quebrado no dir do agente
	managed := agents[0].ManagedDir
	os.MkdirAll(managed, 0o755)
	os.Symlink("/nonexistent/path/skill", filepath.Join(managed, "broken"))

	stdout, _, _ := run(t, []string{"doctor"}, svc, agents)
	if !strings.Contains(stdout, "broken symlink") {
		t.Errorf("doctor deveria reportar symlink quebrado: %q", stdout)
	}
}

// `skills <sub>` é o mesmo comando do topo; sem sub, lista.
func TestCLISkillsGroup(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "my-skill", strings.Repeat("descrição longa ", 20))
	for _, args := range [][]string{{"skills"}, {"skills", "list"}} {
		stdout, stderr, code := run(t, args, svc, agents)
		if code != 0 || !strings.Contains(stdout, "my-skill") {
			t.Fatalf("%v = exit %d, %q %q", args, code, stdout, stderr)
		}
		if strings.Count(stdout, "descrição longa") > 4 || !strings.Contains(stdout, "…") {
			t.Errorf("descrição não truncada: %q", stdout)
		}
	}
	if _, _, code := run(t, []string{"skills", "xyz"}, svc, agents); code != 1 {
		t.Errorf("subcomando desconhecido = %d", code)
	}
}
