package skill

import (
	"os"
	"path/filepath"
	"testing"

	"lazyskills/internal/agent"
)

// testPaths cria Paths isolados num TempDir — nunca toca o home real.
func testPaths(t *testing.T) Paths {
	t.Helper()
	home := t.TempDir()
	return Paths{
		Home:      home,
		ConfigDir: filepath.Join(home, ".config", "lazyskills"),
		DataDir:   filepath.Join(home, ".local", "share", "lazyskills"),
	}
}

// writeSkill cria parent/dir/SKILL.md com o conteúdo dado e devolve o caminho da pasta.
func writeSkill(t *testing.T, parent, dir, md string) string {
	t.Helper()
	path := filepath.Join(parent, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("criando %s: %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("escrevendo SKILL.md em %s: %v", path, err)
	}
	return path
}

func validMD(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n\ncorpo\n"
}

func byDirMap(skills []Skill) map[string]Skill {
	m := make(map[string]Skill, len(skills))
	for _, sk := range skills {
		m[sk.Dir] = sk
	}
	return m
}

func mustSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(newname), 0o755); err != nil {
		t.Fatalf("criando dir pai de %s: %v", newname, err)
	}
	if err := os.Symlink(oldname, newname); err != nil {
		t.Fatalf("symlink %s -> %s: %v", newname, oldname, err)
	}
}

func TestParseMeta(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantOK   bool
		wantMeta Meta
	}{
		{
			name:     "frontmatter válido",
			data:     "---\nname: foo\ndescription: bar\n---\ncorpo\n",
			wantOK:   true,
			wantMeta: Meta{Name: "foo", Description: "bar"},
		},
		{
			name:   "sem frontmatter",
			data:   "# só markdown\n\ntexto\n",
			wantOK: false,
		},
		{
			name:   "yaml inválido",
			data:   "---\nname: [aberto\n---\ncorpo\n",
			wantOK: false,
		},
		{
			name:     "com BOM UTF-8",
			data:     "\xef\xbb\xbf---\nname: foo\n---\ncorpo\n",
			wantOK:   true,
			wantMeta: Meta{Name: "foo"},
		},
		{
			name:     "cerca de fechamento reticências",
			data:     "---\nname: foo\n...\ncorpo\n",
			wantOK:   true,
			wantMeta: Meta{Name: "foo"},
		},
		{
			name:   "cerca sem fechamento",
			data:   "---\nname: foo\n",
			wantOK: false,
		},
		{
			name:   "vazio",
			data:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := ParseMeta([]byte(tt.data))
			if ok != tt.wantOK {
				t.Fatalf("ParseMeta ok = %v, quer %v", ok, tt.wantOK)
			}
			if meta != tt.wantMeta {
				t.Errorf("ParseMeta meta = %+v, quer %+v", meta, tt.wantMeta)
			}
		})
	}
}

func TestScan(t *testing.T) {
	paths := testPaths(t)
	svc := New(paths)
	lib := paths.LibraryDir()

	writeSkill(t, lib, "alpha", validMD("Alpha Skill", "skill válida"))
	writeSkill(t, lib, "beta", validMD("beta", "compartilhada"))
	writeSkill(t, lib, "broken", "# sem frontmatter\n")

	managed := filepath.Join(paths.Home, "agents", "ag1", "skills")
	shared := filepath.Join(paths.Home, "agents", "shared", "skills")

	// dir real no agente → Local
	writeSkill(t, managed, "local-sk", validMD("local-sk", "local"))
	// symlink gerenciado (ManagedDir → biblioteca) → Managed
	mustSymlink(t, filepath.Join(lib, "alpha"), filepath.Join(managed, "alpha"))
	// symlink de terceiros (aponta para fora da biblioteca) → Local
	foreign := writeSkill(t, filepath.Join(paths.Home, "elsewhere"), "foreign", validMD("foreign", "de fora"))
	mustSymlink(t, foreign, filepath.Join(managed, "foreign"))
	// symlink para a biblioteca num segundo ReadDir compartilhado → On sem Managed
	mustSymlink(t, filepath.Join(lib, "beta"), filepath.Join(shared, "beta"))

	agents := []agent.Agent{
		{ID: "ag1", Name: "Ag1", Short: "1", Installed: true, ManagedDir: managed, ReadDirs: []string{managed, shared}},
		{ID: "off", Name: "Off", Short: "0", Installed: false, ManagedDir: managed, ReadDirs: []string{managed}},
	}

	skills, err := svc.Scan(agents)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	m := byDirMap(skills)

	tests := []struct {
		dir       string
		inLibrary bool
		valid     bool
		hasState  bool
		state     AgentState
	}{
		{"alpha", true, true, true, AgentState{On: true, Managed: true, Via: managed}},
		{"beta", true, true, true, AgentState{On: true, Via: shared}},
		{"broken", true, false, false, AgentState{}},
		{"local-sk", false, true, true, AgentState{On: true, Local: true, Via: managed}},
		{"foreign", false, true, true, AgentState{On: true, Local: true, Via: managed}},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			sk, ok := m[tt.dir]
			if !ok {
				t.Fatalf("skill %s não encontrada no scan", tt.dir)
			}
			if sk.InLibrary != tt.inLibrary {
				t.Errorf("InLibrary = %v, quer %v", sk.InLibrary, tt.inLibrary)
			}
			if sk.Valid != tt.valid {
				t.Errorf("Valid = %v, quer %v", sk.Valid, tt.valid)
			}
			st, ok := sk.States["ag1"]
			if ok != tt.hasState {
				t.Fatalf("States[ag1] presente = %v, quer %v", ok, tt.hasState)
			}
			if st != tt.state {
				t.Errorf("States[ag1] = %+v, quer %+v", st, tt.state)
			}
		})
	}

	if got := m["alpha"].Name; got != "Alpha Skill" {
		t.Errorf("Name de alpha = %q, quer frontmatter %q", got, "Alpha Skill")
	}
	if got := m["alpha"].EnabledCount(); got != 1 {
		t.Errorf("EnabledCount de alpha = %d, quer 1", got)
	}
	if m["broken"].Warning == "" {
		t.Error("skill inválida deveria carregar Warning")
	}
	for _, sk := range skills {
		if _, ok := sk.States["off"]; ok {
			t.Errorf("agente não instalado não deveria gerar estado (skill %s)", sk.Dir)
		}
	}
}
