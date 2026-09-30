package agent

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// Crush adapts Charm's Crush. Its global config is ~/.config/crush and its
// global data ~/.local/share/crush; sessions live per project (crush_sessions.go).
type Crush struct {
	Home string
	// XDG_CONFIG_HOME, XDG_DATA_HOME, CRUSH_GLOBAL_DATA and CRUSH_SKILLS_DIR;
	// empty means the defaults.
	ConfigHome, DataHome, GlobalData, SkillsDir string
	Look                                        func(string) (string, error)
}

func NewCrush(home string) *Crush {
	return &Crush{Home: home, ConfigHome: envPath(home, "XDG_CONFIG_HOME"), DataHome: envPath(home, "XDG_DATA_HOME"),
		GlobalData: envPath(home, "CRUSH_GLOBAL_DATA"), SkillsDir: envPath(home, "CRUSH_SKILLS_DIR"), Look: exec.LookPath}
}

func (c *Crush) ID() string { return "crush" }

func (c *Crush) configBase() string {
	if c.ConfigHome != "" {
		return c.ConfigHome
	}
	return filepath.Join(c.Home, ".config")
}

func (c *Crush) configDir() string { return filepath.Join(c.configBase(), "crush") }

func (c *Crush) dataDir() string {
	switch {
	case c.GlobalData != "":
		return c.GlobalData
	case c.DataHome != "":
		return filepath.Join(c.DataHome, "crush")
	}
	return filepath.Join(c.Home, ".local", "share", "crush")
}

// Detect announces the skill dirs Crush 0.96 was seen loading: its own, the
// cross-agent ones and ~/.claude/skills. CRUSH_SKILLS_DIR replaces all of
// them, so it is then the only one.
func (c *Crush) Detect() Agent {
	a := Agent{ID: "crush", Name: "Crush", Short: "R"}
	bin, _ := c.Look("crush")
	a.Installed = bin != "" || dirExists(c.configDir()) || dirExists(c.dataDir())
	if !a.Installed {
		a.Detail = DetailNotInstalled
		return a
	}
	if c.SkillsDir != "" {
		a.ManagedDir = c.SkillsDir
		a.ReadDirs = []string{a.ManagedDir}
	} else {
		a.ManagedDir = filepath.Join(c.configDir(), "skills")
		a.SharedDir = filepath.Join(c.Home, ".agents", "skills")
		a.ReadDirs = []string{a.ManagedDir, filepath.Join(c.configBase(), "agents", "skills"),
			filepath.Join(c.Home, ".claude", "skills"), a.SharedDir}
	}
	if bin != "" {
		a.Version = version(bin)
		a.Detail = bin
	} else {
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", c.configDir())
	}
	return a
}
