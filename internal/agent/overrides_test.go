package agent

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestMain clears the CLI overrides so no test reads the developer's own.
func TestMain(m *testing.M) {
	for _, o := range ConfigOverrides {
		os.Unsetenv(o.Var)
	}
	os.Exit(m.Run())
}

// Every variable in ConfigOverrides moves where its adapter looks; a relative
// value is ignored.
func TestConfigOverrides(t *testing.T) {
	home := t.TempDir()
	crushSkills := func() string {
		a := (&Crush{Home: home, SkillsDir: envPath(home, "CRUSH_SKILLS_DIR"), Look: noBin}).Detect()
		return a.ManagedDir
	}
	crush := map[string]func() string{
		"XDG_CONFIG_HOME":   func() string { return filepath.Dir(NewCrush(home).configDir()) },
		"XDG_DATA_HOME":     func() string { return filepath.Dir(NewCrush(home).dataDir()) },
		"CRUSH_GLOBAL_DATA": func() string { return NewCrush(home).dataDir() },
		"CRUSH_GLOBAL_CONFIG": func() string {
			if c := NewCrush(home); c.GlobalConfig != "" {
				return filepath.Dir(c.configFile())
			}
			return ""
		},
		"CRUSH_SKILLS_DIR": func() string { t.Helper(); mkdirs(t, filepath.Join(home, ".config", "crush")); return crushSkills() },
	}
	where := map[string]func() string{
		"CLAUDE_CONFIG_DIR":           func() string { return NewClaude(home).configDir() },
		"CODEX_HOME":                  func() string { return NewCodex(home).configDir() },
		"XDG_CONFIG_HOME":             func() string { return filepath.Dir(NewOpenCode(home).configDir()) },
		"XDG_DATA_HOME":               func() string { return filepath.Dir(filepath.Dir(NewOpenCode(home).dbPath())) },
		"OPENCODE_DB":                 func() string { return NewOpenCode(home).dbPath() },
		"HERMES_HOME":                 func() string { return NewHermes(home).configDir() },
		"LOCALAPPDATA":                func() string { return filepath.Dir(NewHermes(home).configDir()) },
		"PI_CODING_AGENT_DIR":         func() string { return NewPi(home).agentDir() },
		"PI_CODING_AGENT_SESSION_DIR": func() string { return NewPi(home).sessionsDir() },
	}
	for _, o := range ConfigOverrides {
		get, ok := where[o.Var]
		if o.Agent == "crush" {
			get, ok = crush[o.Var]
		}
		if !ok {
			t.Errorf("%s: no test for this override", o.Var)
			continue
		}
		if o.Var == "LOCALAPPDATA" && runtime.GOOS != "windows" {
			continue // only read on Windows
		}
		def := get()
		abs := filepath.Join(t.TempDir(), "moved")
		t.Setenv(o.Var, abs)
		if got := get(); got != abs {
			t.Errorf("%s=%s: adapter looks in %s", o.Var, abs, got)
		}
		t.Setenv(o.Var, "~/moved/")
		if got := get(); got != filepath.Join(home, "moved") {
			t.Errorf("%s=~/moved/: adapter looks in %s", o.Var, got)
		}
		t.Setenv(o.Var, "relative/dir")
		if got := get(); got != def {
			t.Errorf("%s=relative/dir: %s, want the default %s", o.Var, got, def)
		}
		t.Setenv(o.Var, "")
	}
}

// CODEX_HOME moves Codex's own skills dir; the shared ~/.agents/skills stays.
func TestCodexHomeSkills(t *testing.T) {
	home, codexHome := t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	c := NewCodex(home)
	c.Look = noBin
	a := c.Detect()
	if !a.Installed || a.ManagedDir != filepath.Join(codexHome, "skills") || a.SharedDir != filepath.Join(home, ".agents", "skills") {
		t.Errorf("Codex with CODEX_HOME: %+v", a)
	}
}

// A test package that builds real adapters must clear ConfigOverrides first
// (a TestMain), or a developer's CODEX_HOME would point it at real files.
func TestAdapterTestsClearOverrides(t *testing.T) {
	builds := regexp.MustCompile(`agent\.(New[A-Z]\w*|All|AllWithIndex)\(|app\.Load|LoadWith\(`)
	uses := map[string]bool{}
	clears := map[string]bool{}
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		if builds.Match(data) {
			uses[dir] = true
		}
		if strings.Contains(string(data), "func TestMain") &&
			(strings.Contains(string(data), "agenttest.ClearOverrides") || strings.Contains(string(data), "ConfigOverrides")) {
			clears[dir] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for dir := range uses {
		if !clears[dir] {
			t.Errorf("%s builds adapters in tests but no TestMain calls agenttest.ClearOverrides", dir)
		}
	}
}
