package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// testPaths returns core.Paths isolated in a TempDir; never the real home.
func testPaths(t *testing.T) core.Paths {
	t.Helper()
	return core.PathsIn(t.TempDir())
}

// writeSkill creates parent/dir/SKILL.md with md and returns the folder path.
func writeSkill(t *testing.T, parent, dir, md string) string {
	t.Helper()
	path := filepath.Join(parent, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("writing SKILL.md in %s: %v", path, err)
	}
	return path
}

func validMD(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n\nbody\n"
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
		t.Fatalf("creating parent dir of %s: %v", newname, err)
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
			name:     "valid frontmatter",
			data:     "---\nname: foo\ndescription: bar\n---\nbody\n",
			wantOK:   true,
			wantMeta: Meta{Name: "foo", Description: "bar"},
		},
		{
			name:   "no frontmatter",
			data:   "# markdown only\n\ntext\n",
			wantOK: false,
		},
		{
			name:   "invalid yaml",
			data:   "---\nname: [open\n---\nbody\n",
			wantOK: false,
		},
		{
			name:     "with UTF-8 BOM",
			data:     "\xef\xbb\xbf---\nname: foo\n---\nbody\n",
			wantOK:   true,
			wantMeta: Meta{Name: "foo"},
		},
		{
			name:     "ellipsis closing fence",
			data:     "---\nname: foo\n...\nbody\n",
			wantOK:   true,
			wantMeta: Meta{Name: "foo"},
		},
		{
			name:   "unclosed fence",
			data:   "---\nname: foo\n",
			wantOK: false,
		},
		{
			name:   "empty",
			data:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := ParseMeta([]byte(tt.data))
			if ok != tt.wantOK {
				t.Fatalf("ParseMeta ok = %v, want %v", ok, tt.wantOK)
			}
			if meta != tt.wantMeta {
				t.Errorf("ParseMeta meta = %+v, want %+v", meta, tt.wantMeta)
			}
		})
	}
}

func TestScan(t *testing.T) {
	paths := testPaths(t)
	svc := New(paths)
	lib := paths.LibraryDir()

	writeSkill(t, lib, "alpha", validMD("Alpha Skill", "valid skill"))
	writeSkill(t, lib, "beta", validMD("beta", "shared"))
	writeSkill(t, lib, "broken", "# no frontmatter\n")

	managed := filepath.Join(paths.Home, "agents", "ag1", "skills")
	shared := filepath.Join(paths.Home, "agents", "shared", "skills")

	// real dir in the agent → Local
	writeSkill(t, managed, "local-sk", validMD("local-sk", "local"))
	// managed symlink (ManagedDir → library) → Managed
	mustSymlink(t, filepath.Join(lib, "alpha"), filepath.Join(managed, "alpha"))
	// third-party symlink (outside the library) → Local
	foreign := writeSkill(t, filepath.Join(paths.Home, "elsewhere"), "foreign", validMD("foreign", "from elsewhere"))
	mustSymlink(t, foreign, filepath.Join(managed, "foreign"))
	// library symlink in a second, shared ReadDir → On without Managed
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
				t.Fatalf("skill %s not found by the scan", tt.dir)
			}
			if sk.InLibrary != tt.inLibrary {
				t.Errorf("InLibrary = %v, want %v", sk.InLibrary, tt.inLibrary)
			}
			if sk.Valid != tt.valid {
				t.Errorf("Valid = %v, want %v", sk.Valid, tt.valid)
			}
			st, ok := sk.States["ag1"]
			if ok != tt.hasState {
				t.Fatalf("States[ag1] present = %v, want %v", ok, tt.hasState)
			}
			if st != tt.state {
				t.Errorf("States[ag1] = %+v, want %+v", st, tt.state)
			}
		})
	}

	if got := m["alpha"].Name; got != "Alpha Skill" {
		t.Errorf("alpha Name = %q, want frontmatter %q", got, "Alpha Skill")
	}
	if got := m["alpha"].EnabledCount(); got != 1 {
		t.Errorf("alpha EnabledCount = %d, want 1", got)
	}
	if m["broken"].Warning == "" {
		t.Error("invalid skill should carry a Warning")
	}
	for _, sk := range skills {
		if _, ok := sk.States["off"]; ok {
			t.Errorf("agent not installed should have no state (skill %s)", sk.Dir)
		}
	}
}
