package agent

import (
	"path/filepath"
	"slices"
	"testing"
)

// The dirs Crush 0.96 was seen loading skills from; CRUSH_SKILLS_DIR alone
// replaces them.
func TestCrushDetect(t *testing.T) {
	home := t.TempDir()
	if a := (&Crush{Home: home, Look: noBin}).Detect(); a.Installed || a.SupportsSkills() {
		t.Errorf("crush in an empty home: %+v", a)
	}
	mkdirs(t, filepath.Join(home, ".config", "crush"))
	a := (&Crush{Home: home, Look: noBin}).Detect()
	own, shared := filepath.Join(home, ".config", "crush", "skills"), filepath.Join(home, ".agents", "skills")
	want := []string{own, filepath.Join(home, ".config", "agents", "skills"), filepath.Join(home, ".claude", "skills"), shared}
	if !a.Installed || a.ID != "crush" || a.Short != "R" || a.ManagedDir != own || a.SharedDir != shared || !slices.Equal(a.ReadDirs, want) {
		t.Errorf("Detect = %+v", a)
	}
	only := filepath.Join(home, "skills-only")
	a = (&Crush{Home: home, SkillsDir: only, Look: noBin}).Detect()
	if a.ManagedDir != only || a.SharedDir != "" || !slices.Equal(a.ReadDirs, []string{only}) {
		t.Errorf("with CRUSH_SKILLS_DIR: %+v", a)
	}
	// the data dir alone (sessions, no config yet) also means installed
	data := t.TempDir()
	mkdirs(t, filepath.Join(data, ".local", "share", "crush"))
	if a := (&Crush{Home: data, Look: noBin}).Detect(); !a.Installed {
		t.Error("crush with only its data dir should be detected")
	}
}
