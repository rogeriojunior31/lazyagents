package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lazyskills/internal/agent"
)

// enable ativa a skill (por nome de pasta) no agente, rescaneando antes.
func enable(t *testing.T, svc *Service, agents []agent.Agent, dir string, ag agent.Agent) {
	t.Helper()
	sk := scanOne(t, svc, agents, dir)
	if err := svc.Enable(sk, ag); err != nil {
		t.Fatalf("Enable %s em %s: %v", dir, ag.ID, err)
	}
}

func TestSaveGetListProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// arquivo ausente → lista vazia, sem erro
	names, err := svc.ListProfiles()
	if err != nil || len(names) != 0 {
		t.Fatalf("ListProfiles inicial: err=%v names=%v", err, names)
	}

	// salvar dois perfis (spec por agente, com dedupe nas listas de agentes)
	if err := svc.SaveProfile("trabalho", ProfileSpec{
		"sk-a": {"codex", "claude-code", "claude-code"},
		"sk-b": {"claude-code"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveProfile("pessoal", ProfileSpec{"sk-c": {"claude-code"}}); err != nil {
		t.Fatal(err)
	}

	// nome vazio → erro
	if err := svc.SaveProfile("", ProfileSpec{"sk-a": {"claude-code"}}); err == nil {
		t.Fatal("nome vazio deveria falhar")
	}

	names, err = svc.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "pessoal" || names[1] != "trabalho" {
		t.Fatalf("ListProfiles: %v", names)
	}

	spec, err := svc.GetProfile("trabalho")
	if err != nil {
		t.Fatal(err)
	}
	// dedupe + sort dos agentes de sk-a
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Fatalf("GetProfile sk-a agentes: %v", got)
	}
	if got := spec["sk-b"]; !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Fatalf("GetProfile sk-b agentes: %v", got)
	}

	// perfil inexistente → erro
	if _, err := svc.GetProfile("nao-existe"); err == nil {
		t.Fatal("GetProfile inexistente deveria falhar")
	}
}

func TestSaveProfileDropsEmpty(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	if err := svc.SaveProfile("x", ProfileSpec{
		"sk-a": {"claude-code"},
		"sk-b": {},              // sem agentes → descartada
		"":     {"claude-code"}, // nome de skill vazio → descartada
	}); err != nil {
		t.Fatal(err)
	}
	spec, err := svc.GetProfile("x")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec) != 1 || spec["sk-a"] == nil {
		t.Fatalf("entradas vazias não foram descartadas: %v", spec)
	}
}

func TestDeleteProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	if err := svc.SaveProfile("temp", ProfileSpec{"sk-a": {"claude-code"}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteProfile("temp"); err != nil {
		t.Fatal(err)
	}
	names, _ := svc.ListProfiles()
	for _, n := range names {
		if n == "temp" {
			t.Fatal("perfil deveria ter sido deletado")
		}
	}
	// deletar inexistente não é erro
	if err := svc.DeleteProfile("nao-existe"); err != nil {
		t.Fatalf("deletar inexistente: %v", err)
	}
}

func TestProfilesRoundTripUnknownFields(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// gravar JSON com campo desconhecido "version" e um perfil no formato novo
	initial := `{"version": 42, "profiles": {"x": {"sk-a": ["claude-code"]}}}`
	if err := os.MkdirAll(filepath.Dir(p.ProfilesPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfilesPath(), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// salvar um perfil novo — deve preservar "version"
	if err := svc.SaveProfile("y", ProfileSpec{"sk-b": {"codex"}}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(p.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("JSON inválido após save: %v", err)
	}
	if _, ok := raw["version"]; !ok {
		t.Error("campo desconhecido 'version' foi perdido no round-trip")
	}
	if _, ok := raw["profiles"]; !ok {
		t.Error("campo 'profiles' sumiu")
	}
}

// TestProfileLegacyMigration: perfil no formato antigo (lista plana) é lido como
// "todos os agentes" e regravado no formato novo com IDs concretos ao salvar.
func TestProfileLegacyMigration(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc"))

	// formato legado: array de skills
	legacy := `{"profiles": {"velho": ["sk-a"]}}`
	if err := os.MkdirAll(filepath.Dir(p.ProfilesPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfilesPath(), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	// lido como {sk-a: ["*"]}
	spec, err := svc.GetProfile("velho")
	if err != nil {
		t.Fatal(err)
	}
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{allAgents}) {
		t.Fatalf("migração legado: %v", got)
	}

	// aplicar expande "*" para todos os agentes instalados
	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}
	if err := svc.ApplyProfile("velho", agents); err != nil {
		t.Fatalf("ApplyProfile legado: %v", err)
	}
	for _, ag := range agents {
		if _, err := os.Lstat(filepath.Join(ag.ManagedDir, "sk-a")); err != nil {
			t.Errorf("sk-a deveria estar ativa em %s", ag.ID)
		}
	}

	// ao regravar via snapshot, "*" vira IDs concretos
	skills, _ := svc.Scan(agents)
	if err := svc.SaveProfile("velho", BuildProfileSpec(skills, agents)); err != nil {
		t.Fatal(err)
	}
	spec, _ = svc.GetProfile("velho")
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Fatalf("após regravar: %v", got)
	}
}

// TestBuildProfileSpecSnapshot: o snapshot captura os agentes exatos e ignora
// skills locais.
func TestBuildProfileSpecSnapshot(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}

	// sk-a só no claude-code; sk-b nos dois
	enable(t, svc, agents, "sk-a", agC)
	enable(t, svc, agents, "sk-b", agC)
	enable(t, svc, agents, "sk-b", agX)

	skills, _ := svc.Scan(agents)
	spec := BuildProfileSpec(skills, agents)
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Errorf("sk-a snapshot: %v", got)
	}
	if got := spec["sk-b"]; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Errorf("sk-b snapshot: %v", got)
	}
}

// TestApplyProfilePerAgent: aplicar restaura a matriz por agente e é idempotente.
func TestApplyProfilePerAgent(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc a"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc b"))
	writeSkill(t, lib, "sk-c", validMD("sk-c", "desc c"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}

	// perfil: sk-a nos dois, sk-b só no claude-code
	if err := svc.SaveProfile("trabalho", ProfileSpec{
		"sk-a": {"claude-code", "codex"},
		"sk-b": {"claude-code"},
	}); err != nil {
		t.Fatal(err)
	}

	on := func(ag agent.Agent, name string) bool {
		_, err := os.Lstat(filepath.Join(ag.ManagedDir, name))
		return err == nil
	}
	apply := func() {
		t.Helper()
		if err := svc.ApplyProfile("trabalho", agents); err != nil {
			t.Fatalf("ApplyProfile: %v", err)
		}
	}

	// suja a matriz antes: liga sk-c no codex
	enable(t, svc, agents, "sk-c", agX)

	apply()
	check := func() {
		t.Helper()
		if !on(agC, "sk-a") || !on(agX, "sk-a") {
			t.Error("sk-a deveria estar nos dois agentes")
		}
		if !on(agC, "sk-b") || on(agX, "sk-b") {
			t.Error("sk-b deveria estar só no claude-code")
		}
		if on(agC, "sk-c") || on(agX, "sk-c") {
			t.Error("sk-c deveria estar desativada em todos")
		}
	}
	check()

	// idempotente
	apply()
	check()
}

func TestDiffProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc a"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc b"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}

	// perfil quer sk-a nos dois; estado atual: sk-a só no claude-code, sk-b no codex
	if err := svc.SaveProfile("trabalho", ProfileSpec{"sk-a": {"claude-code", "codex"}}); err != nil {
		t.Fatal(err)
	}
	enable(t, svc, agents, "sk-a", agC)
	enable(t, svc, agents, "sk-b", agX)

	changes, err := svc.DiffProfile("trabalho", agents)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]ProfileChange, len(changes))
	for _, c := range changes {
		byName[c.Skill] = c
	}
	if got := byName["sk-a"].Add; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("sk-a Add: %v", got)
	}
	if got := byName["sk-b"].Remove; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("sk-b Remove: %v", got)
	}

	// aplicar e conferir que o diff zera
	if err := svc.ApplyProfile("trabalho", agents); err != nil {
		t.Fatal(err)
	}
	changes, err = svc.DiffProfile("trabalho", agents)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Errorf("diff após aplicar deveria ser vazio: %v", changes)
	}
}

// TestProfileSharedReadEcho: um agente que lê o dir gerenciado de outro (ex.:
// opencode lendo ~/.claude/skills) não é capturado no snapshot nem desativado
// pelo Apply — desativá-lo removeria o symlink do outro agente.
func TestProfileSharedReadEcho(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-a", validMD("sk-a", "desc"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	// opencode: dir gerenciado próprio + lê o do claude
	agO := testAgent(p.Home, "opencode", ".config/opencode/skills", ".claude/skills")
	agents := []agent.Agent{agC, agO}

	// ativa sk-a só no claude; opencode "vê" via dir compartilhado (eco)
	enable(t, svc, agents, "sk-a", agC)

	sk := scanOne(t, svc, agents, "sk-a")
	if !sk.States["claude-code"].Managed {
		t.Fatal("sk-a deveria ser gerenciada no claude-code")
	}
	if sk.States["opencode"].Managed {
		t.Fatal("eco no opencode não deveria contar como Managed")
	}

	// snapshot captura só o claude
	spec := BuildProfileSpec([]Skill{sk}, agents)
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Fatalf("snapshot com eco: %v", got)
	}

	// perfil que NÃO quer sk-a em ninguém: Apply desativa no claude mas não
	// pode remover o eco (que é o mesmo symlink do claude, já removido).
	if err := svc.SaveProfile("vazio", ProfileSpec{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyProfile("vazio", agents); err != nil {
		t.Fatalf("ApplyProfile vazio: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(agC.ManagedDir, "sk-a")); err == nil {
		t.Error("sk-a deveria ter sido desativada no claude")
	}
}

func TestApplyProfileMissingSkill(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-a", validMD("sk-a", "desc"))

	ag := testAgent(p.Home, "claude-code", ".claude/skills")

	if err := svc.SaveProfile("ruim", ProfileSpec{
		"sk-a":       {"claude-code"},
		"nao-existe": {"claude-code"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyProfile("ruim", []agent.Agent{ag}); err == nil {
		t.Fatal("skill inexistente deveria falhar")
	}
}

func TestApplyProfileNotFound(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	if err := svc.ApplyProfile("nao-existe", nil); err == nil {
		t.Fatal("perfil inexistente deveria falhar")
	}
}
