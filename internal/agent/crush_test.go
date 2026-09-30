package agent

import (
	"os"
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

// Only whether a provider has an api_key is read, from crush.json where
// CRUSH_GLOBAL_CONFIG may have moved it.
func TestCrushAuthMode(t *testing.T) {
	for _, v := range crushKeyEnv { // the developer's own keys must not count
		t.Setenv(v, "")
	}
	home := t.TempDir()
	c := &Crush{Home: home, Look: noBin}
	if mode, _ := c.AuthMode(); mode != AuthUnknown {
		t.Errorf("no config: %v", mode)
	}
	writeFile(t, filepath.Join(home, ".config", "crush", "crush.json"), `{"providers":{"hyper":{"type":"openai-compat"},"openrouter":{"api_key":"$OPENROUTER_API_KEY"}}}`)
	if mode, detail := c.AuthMode(); mode != AuthAPIKey || detail != "openrouter" {
		t.Errorf("api_key set: %v %q", mode, detail)
	}
	moved := t.TempDir()
	c.GlobalConfig = moved
	if mode, _ := c.AuthMode(); mode != AuthUnknown {
		t.Errorf("CRUSH_GLOBAL_CONFIG points at a dir without crush.json: %v", mode)
	}
	// onboarding saves keys in the data dir's crush.json
	writeFile(t, filepath.Join(c.dataDir(), "crush.json"), `{"providers":{"anthropic":{"api_key":"k"}}}`)
	if mode, detail := c.AuthMode(); mode != AuthAPIKey || detail != "anthropic" {
		t.Errorf("key in the data dir: %v %q", mode, detail)
	}
	os.Remove(filepath.Join(c.dataDir(), "crush.json"))
	t.Setenv("OPENROUTER_API_KEY", "k")
	if mode, detail := c.AuthMode(); mode != AuthAPIKey || detail != "OPENROUTER_API_KEY" {
		t.Errorf("key in the environment: %v %q", mode, detail)
	}
}
