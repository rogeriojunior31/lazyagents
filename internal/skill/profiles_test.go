package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lazyskills/internal/agent"
)

func TestSaveGetListProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// arquivo ausente → lista vazia, sem erro
	names, err := svc.ListProfiles()
	if err != nil || len(names) != 0 {
		t.Fatalf("ListProfiles inicial: err=%v names=%v", err, names)
	}

	// salvar dois perfis
	if err := svc.SaveProfile("trabalho", []string{"sk-b", "sk-a", "sk-a"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveProfile("pessoal", []string{"sk-c"}); err != nil {
		t.Fatal(err)
	}

	// nome vazio → erro
	if err := svc.SaveProfile("", []string{"sk-a"}); err == nil {
		t.Fatal("nome vazio deveria falhar")
	}

	names, err = svc.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "pessoal" || names[1] != "trabalho" {
		t.Fatalf("ListProfiles: %v", names)
	}

	skills, err := svc.GetProfile("trabalho")
	if err != nil {
		t.Fatal(err)
	}
	// dedupe + sort: sk-a, sk-b
	if len(skills) != 2 || skills[0] != "sk-a" || skills[1] != "sk-b" {
		t.Fatalf("GetProfile trabalho: %v", skills)
	}

	// perfil inexistente → erro
	if _, err := svc.GetProfile("nao-existe"); err == nil {
		t.Fatal("GetProfile inexistente deveria falhar")
	}
}

func TestDeleteProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	if err := svc.SaveProfile("temp", []string{"sk-a"}); err != nil {
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

	// gravar JSON com campo desconhecido "version"
	initial := `{"version": 42, "profiles": {"x": ["sk-a"]}}`
	if err := os.MkdirAll(filepath.Dir(p.ProfilesPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfilesPath(), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// salvar um perfil novo — deve preservar "version"
	if err := svc.SaveProfile("y", []string{"sk-b"}); err != nil {
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

func TestApplyProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()

	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc a"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc b"))
	writeSkill(t, lib, "sk-c", validMD("sk-c", "desc c"))

	ag := testAgent(p.Home, "claude-code", ".claude/skills")
	agents := []agent.Agent{ag}

	if err := svc.SaveProfile("trabalho", []string{"sk-a", "sk-b"}); err != nil {
		t.Fatal(err)
	}

	// aplicar
	if err := svc.ApplyProfile("trabalho", agents); err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	// sk-a e sk-b ativas, sk-c inativa
	checkLink := func(name string, wantOn bool) {
		t.Helper()
		link := filepath.Join(ag.ManagedDir, name)
		_, err := os.Lstat(link)
		on := err == nil
		if on != wantOn {
			t.Errorf("skill %s: on=%v want=%v", name, on, wantOn)
		}
	}
	checkLink("sk-a", true)
	checkLink("sk-b", true)
	checkLink("sk-c", false)

	// idempotente: aplicar de novo não quebra
	if err := svc.ApplyProfile("trabalho", agents); err != nil {
		t.Fatalf("ApplyProfile idempotente: %v", err)
	}
	checkLink("sk-a", true)
	checkLink("sk-b", true)

	// trocar perfil: desativa sk-b, ativa sk-c
	if err := svc.SaveProfile("pessoal", []string{"sk-a", "sk-c"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyProfile("pessoal", agents); err != nil {
		t.Fatalf("ApplyProfile pessoal: %v", err)
	}
	checkLink("sk-a", true)
	checkLink("sk-b", false)
	checkLink("sk-c", true)
}

func TestApplyProfileMissingSkill(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-a", validMD("sk-a", "desc"))

	ag := testAgent(p.Home, "claude-code", ".claude/skills")

	if err := svc.SaveProfile("ruim", []string{"sk-a", "nao-existe"}); err != nil {
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
