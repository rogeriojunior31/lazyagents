package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Crush adapts Charm's Crush. Its global config is ~/.config/crush and its
// global data ~/.local/share/crush; sessions live per project (crush_sessions.go).
type Crush struct {
	Home string
	// XDG_CONFIG_HOME, XDG_DATA_HOME, CRUSH_GLOBAL_CONFIG, CRUSH_GLOBAL_DATA and
	// CRUSH_SKILLS_DIR; empty means the defaults.
	ConfigHome, DataHome, GlobalConfig, GlobalData, SkillsDir string
	Look                                                      func(string) (string, error)
}

func NewCrush(home string) *Crush {
	return &Crush{Home: home, ConfigHome: envPath(home, "XDG_CONFIG_HOME"), DataHome: envPath(home, "XDG_DATA_HOME"),
		GlobalConfig: envPath(home, "CRUSH_GLOBAL_CONFIG"), GlobalData: envPath(home, "CRUSH_GLOBAL_DATA"),
		SkillsDir: envPath(home, "CRUSH_SKILLS_DIR"), Look: exec.LookPath}
}

// configFile is crush.json; CRUSH_GLOBAL_CONFIG moves it (and only it: the
// skills stay in the config dir).
func (c *Crush) configFile() string {
	if c.GlobalConfig != "" {
		return filepath.Join(c.GlobalConfig, "crush.json")
	}
	return filepath.Join(c.configDir(), "crush.json")
}

// crushKeyEnv are the provider key variables crush 0.96.1 reads (from its
// binary): a key there bills per token like one in crush.json.
var crushKeyEnv = []string{"AIHUBMIX_API_KEY", "ALIBABA_SINGAPORE_API_KEY", "ALIBABA_US_API_KEY", "ANTHROPIC_API_KEY",
	"ATLASCLOUD_API_KEY", "AVIAN_API_KEY", "AZURE_OPENAI_API_KEY", "BASETEN_API_KEY", "CEREBRAS_API_KEY", "CHUTES_API_KEY",
	"CORALBRICKS_API_KEY", "CORTECS_API_KEY", "DEEPSEEK_API_KEY", "FIREWORKS_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
	"GROQ_API_KEY", "IONET_API_KEY", "KIMI_CODING_API_KEY", "MINIMAX_API_KEY", "MOONSHOT_API_KEY", "NEBIUS_API_KEY",
	"NEURALWATT_API_KEY", "OPENAI_API_KEY", "OPENCODE_API_KEY", "OPENROUTER_API_KEY", "QINIUCLOUD_API_KEY",
	"SYNTHETIC_API_KEY", "VENICE_API_KEY", "VERCEL_API_KEY", "XAI_API_KEY", "ZAI_API_KEY", "ZHIPU_API_KEY"}

// AuthMode is API key when Crush has a provider key: an api_key in crush.json
// (the config one, or the data dir one where onboarding saves keys) or a
// provider key variable. Only whether one is set is read, never its value.
func (c *Crush) AuthMode() (AuthMode, string) {
	for _, file := range []string{c.configFile(), filepath.Join(c.dataDir(), "crush.json")} {
		var cfg struct {
			Providers map[string]struct {
				APIKey secret `json:"api_key"`
			} `json:"providers"`
		}
		if decodeJSONFile(file, &cfg) != nil {
			continue
		}
		for id, p := range cfg.Providers {
			if p.APIKey {
				return AuthAPIKey, id
			}
		}
	}
	for _, v := range crushKeyEnv {
		if os.Getenv(v) != "" {
			return AuthAPIKey, v
		}
	}
	return AuthUnknown, ""
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
