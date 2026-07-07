package skill

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lazyskills/internal/agent"
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
	t.Fatalf("skill %s não apareceu no scan", dir)
	return Skill{}
}

func TestCreate(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	path, err := svc.Create("minha-skill")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := ParseMeta(data)
	if !ok || meta.Name != "minha-skill" || meta.Description == "" {
		t.Fatalf("template inválido: ok=%v meta=%+v", ok, meta)
	}
	// aparece no scan como skill da biblioteca
	sk := scanOne(t, svc, nil, "minha-skill")
	if !sk.InLibrary || !sk.Valid {
		t.Fatalf("skill criada: %+v", sk)
	}
	// duplicada
	if _, err := svc.Create("minha-skill"); err == nil {
		t.Fatal("nome duplicado deveria falhar")
	}
	// nomes inválidos
	for _, bad := range []string{"", "Maiúscula", "com espaço", "-começa-hifen", "termina-", "a_b", "a/b"} {
		if _, err := svc.Create(bad); err == nil {
			t.Errorf("nome %q deveria ser inválido", bad)
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
		t.Fatalf("symlink errado: %q, %v", target, err)
	}

	sk = scanOne(t, svc, agents, "sk")
	if st := sk.States[ag.ID]; !st.On || !st.Managed {
		t.Fatalf("estado pós-enable: %+v", st)
	}
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatalf("enable deveria ser idempotente: %v", err)
	}
	if err := svc.Disable(sk, ag); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("symlink deveria ter sumido: %v", err)
	}

	// dir real: disable recusa e não toca no conteúdo
	writeSkill(t, ag.ManagedDir, "real", validMD("real", "local"))
	real := scanOne(t, svc, agents, "real")
	if err := svc.Disable(real, ag); err == nil {
		t.Fatal("disable de dir real deveria falhar")
	}
	if _, err := os.Stat(filepath.Join(ag.ManagedDir, "real", "SKILL.md")); err != nil {
		t.Fatalf("conteúdo local foi tocado: %v", err)
	}

	// skill fora da biblioteca não pode ser ativada em outro agente
	if err := svc.Enable(real, testAgent(p.Home, "codex", ".agents/skills")); err == nil {
		t.Fatal("enable fora da biblioteca deveria falhar")
	}
	// agente sem ManagedDir
	if err := svc.Enable(sk, agent.Agent{ID: "desktop", Name: "desktop", Installed: true}); err == nil {
		t.Fatal("enable sem ManagedDir deveria falhar")
	}
	// nome já ocupado por dir real no agente
	writeSkill(t, p.LibraryDir(), "real", validMD("real", "na lib"))
	realLib := scanOne(t, svc, agents, "real")
	realLib.InLibrary = true
	realLib.States = map[string]AgentState{} // força caminho do Lstat
	if err := svc.Enable(realLib, ag); err == nil {
		t.Fatal("enable sobre dir real existente deveria falhar")
	}
}

func TestEnableAllDisableAll(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "d"))
	a1 := testAgent(p.Home, "claude-code", ".claude/skills")
	a2 := testAgent(p.Home, "codex", ".agents/skills")
	agents := []agent.Agent{a1, a2, {ID: "sem-skills", Name: "x", Installed: true}}

	sk := scanOne(t, svc, agents, "sk")
	if err := svc.EnableAll(sk, agents); err != nil {
		t.Fatal(err)
	}
	for _, ag := range []agent.Agent{a1, a2} {
		if _, err := os.Lstat(filepath.Join(ag.ManagedDir, "sk")); err != nil {
			t.Errorf("faltou link em %s: %v", ag.ID, err)
		}
	}

	// skill local em outro agente não é tocada pelo DisableAll
	a3 := testAgent(p.Home, "gemini-cli", ".gemini/skills")
	writeSkill(t, a3.ManagedDir, "sk", validMD("sk", "local"))
	all := []agent.Agent{a1, a2, a3}
	sk = scanOne(t, svc, all, "sk")
	if err := svc.DisableAll(sk, all); err != nil {
		t.Fatal(err)
	}
	for _, ag := range []agent.Agent{a1, a2} {
		if _, err := os.Lstat(filepath.Join(ag.ManagedDir, "sk")); !os.IsNotExist(err) {
			t.Errorf("sobrou link em %s", ag.ID)
		}
	}
	if _, err := os.Stat(filepath.Join(a3.ManagedDir, "sk", "SKILL.md")); err != nil {
		t.Errorf("skill local não deveria ser tocada: %v", err)
	}
}

func TestAdopt(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}
	writeSkill(t, ag.ManagedDir, "minha", validMD("minha", "local"))

	sk := scanOne(t, svc, agents, "minha")
	if err := svc.Adopt(sk, ag); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "minha", "SKILL.md")); err != nil {
		t.Fatalf("não copiou pra biblioteca: %v", err)
	}
	link := filepath.Join(ag.ManagedDir, "minha")
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(p.LibraryDir(), "minha") {
		t.Fatalf("origem não virou symlink: %q, %v", target, err)
	}
	if backups, _ := os.ReadDir(p.BackupsDir()); len(backups) != 1 {
		t.Fatalf("esperava 1 backup, tem %d", len(backups))
	}
	if o := readOrigin(filepath.Join(p.LibraryDir(), "minha")); o == nil || o.Type != "dir" {
		t.Fatalf("adopt deveria registrar origem dir: %+v", o)
	}
	// segunda adoção: já está na biblioteca
	sk = scanOne(t, svc, agents, "minha")
	if err := svc.Adopt(sk, ag); err == nil {
		t.Fatal("re-adopt deveria falhar")
	}
	// origem symlink de terceiros é recusada
	foreign := writeSkill(t, t.TempDir(), "alheia", validMD("alheia", "x"))
	mustSymlink(t, foreign, filepath.Join(ag.ManagedDir, "alheia"))
	al := scanOne(t, svc, agents, "alheia")
	if err := svc.Adopt(al, ag); err == nil {
		t.Fatal("adopt de symlink alheio deveria falhar")
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
	// symlink de terceiros homônimo não pode ser removido
	foreign := writeSkill(t, t.TempDir(), "sk", validMD("sk", "de fora"))
	foreignLink := filepath.Join(p.Home, ".gemini", "skills", "sk")
	mustSymlink(t, foreign, foreignLink)
	third := testAgent(p.Home, "gemini-cli", ".gemini/skills")
	all := []agent.Agent{ag, other, third}

	sk = scanOne(t, svc, all, "sk")
	if err := svc.Remove(sk, all); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.LibraryDir(), "sk")); !os.IsNotExist(err) {
		t.Fatal("skill deveria sair da biblioteca")
	}
	for _, a := range []agent.Agent{ag, other} {
		if _, err := os.Lstat(filepath.Join(a.ManagedDir, "sk")); !os.IsNotExist(err) {
			t.Errorf("sobrou link gerenciado em %s", a.ID)
		}
	}
	if _, err := os.Lstat(foreignLink); err != nil {
		t.Error("symlink de terceiros não deveria ser tocado")
	}
	if backups, _ := os.ReadDir(p.BackupsDir()); len(backups) != 1 {
		t.Fatalf("esperava backup do remove, tem %d", len(backups))
	}
	// remover skill fora da biblioteca
	writeSkill(t, ag.ManagedDir, "local", validMD("local", "x"))
	loc := scanOne(t, svc, all, "local")
	if err := svc.Remove(loc, all); err == nil {
		t.Fatal("remove fora da biblioteca deveria falhar")
	}
}

func TestUpdate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não encontrado no PATH")
	}
	p := testPaths(t)
	svc := New(p)
	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}

	// cria repo git local com uma skill
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
	writeSkill(t, repoDir, "minha-skill", validMD("minha-skill", "versão 1"))
	gitExec("add", ".")
	gitExec("commit", "-m", "v1")

	// instala via URL file:// (cloneShallow aceita URLs locais)
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
	if len(names) != 1 || names[0] != "minha-skill" {
		t.Fatalf("Install names = %v", names)
	}

	// ativa no agente para verificar que o symlink sobrevive ao update
	sk := scanOne(t, svc, agents, "minha-skill")
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(ag.ManagedDir, "minha-skill")

	// atualiza o repo com nova versão da skill
	if err := os.WriteFile(filepath.Join(repoDir, "minha-skill", "SKILL.md"),
		[]byte(validMD("minha-skill", "versão 2")), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec("add", ".")
	gitExec("commit", "-m", "v2")

	// executa o update
	sk = scanOne(t, svc, agents, "minha-skill")
	if err := svc.Update(sk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// conteúdo foi atualizado
	data, err := os.ReadFile(filepath.Join(p.LibraryDir(), "minha-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("lendo SKILL.md após update: %v", err)
	}
	if !strings.Contains(string(data), "versão 2") {
		t.Errorf("conteúdo não atualizado: %s", data)
	}

	// backup foi criado
	backups, _ := os.ReadDir(p.BackupsDir())
	if len(backups) != 1 {
		t.Fatalf("esperava 1 backup após update, tem %d", len(backups))
	}

	// symlink de ativação sobreviveu
	if _, err := os.Lstat(linkPath); err != nil {
		t.Errorf("symlink sumiu após update: %v", err)
	}

	// .origin.json preservado com InstalledAt atualizado
	o := readOrigin(filepath.Join(p.LibraryDir(), "minha-skill"))
	if o == nil || o.Type != "git" {
		t.Fatalf("origem perdida após update: %+v", o)
	}
	if o.InstalledAt.IsZero() {
		t.Error("InstalledAt não foi atualizado")
	}

	// skill sem origem git retorna ErrNoGitOrigin
	writeSkill(t, p.LibraryDir(), "manual", validMD("manual", "sem origem"))
	manual := scanOne(t, svc, agents, "manual")
	if err := svc.Update(manual); !errors.Is(err, ErrNoGitOrigin) {
		t.Errorf("update sem origem: quer ErrNoGitOrigin, got %v", err)
	}
}
