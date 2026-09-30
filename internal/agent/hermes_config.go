package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Hermes resolves its home like hermes_constants.py does: HERMES_HOME, else
// ~/.hermes; with no HERMES_HOME, the profile picked by `hermes profile use`
// (<root>/active_profile) is ~/.hermes/profiles/<name>.
func (h *Hermes) configDir() string {
	if v := strings.TrimSpace(h.HermesHome); v != "" {
		if dir := expandPath(h.Home, hermesExpandVars(v)); dir != "" {
			return dir
		}
	}
	root := filepath.Join(h.Home, ".hermes")
	data, err := os.ReadFile(filepath.Join(root, "active_profile"))
	if err != nil {
		return root
	}
	name := strings.TrimSpace(string(data))
	if name == "" || name == "default" || name != filepath.Base(name) {
		return root
	}
	if dir := filepath.Join(root, "profiles", name); dirExists(dir) {
		return dir
	}
	return root
}

// hermesConfig is the part of <hermes home>/config.yaml lazyagents reads.
type hermesConfig struct {
	Skills struct {
		CreateDir    any `yaml:"create_dir"`    // where Hermes writes new skills
		ExternalDirs any `yaml:"external_dirs"` // extra dirs it reads; a list or one string
	} `yaml:"skills"`
}

// skillDirs are the dirs Hermes loads skills from besides <home>/skills, in
// its order (create_dir, then external_dirs), as it resolves them in
// agent/skill_utils.py: ${VAR} and ~ expanded, relative to the Hermes home,
// symlinks resolved, missing dirs and duplicates dropped. A broken config
// gives none: never announce a dir Hermes would not load.
func (h *Hermes) skillDirs(home, local string) []string {
	data, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		return nil
	}
	var cfg hermesConfig
	if yaml.Unmarshal(data, &cfg) != nil {
		return nil
	}
	seen := map[string]bool{}
	if real, err := filepath.EvalSymlinks(local); err == nil {
		seen[real] = true
	}
	var out []string
	for _, entry := range append(hermesStrings(cfg.Skills.CreateDir), hermesStrings(cfg.Skills.ExternalDirs)...) {
		p := hermesExpandVars(entry)
		if p == "~" || strings.HasPrefix(p, "~/") {
			p = filepath.Join(h.Home, p[1:])
		} else if !filepath.IsAbs(p) {
			p = filepath.Join(home, p)
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil || !dirExists(real) || seen[real] {
			continue
		}
		seen[real] = true
		out = append(out, real)
	}
	return out
}

// hermesStrings reads a scalar-or-list config entry as non-empty strings.
func hermesStrings(v any) []string {
	var raw []any
	switch x := v.(type) {
	case string:
		raw = []any{x}
	case []any:
		raw = x
	}
	var out []string
	for _, e := range raw {
		if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// hermesExpandVars is Python's os.path.expandvars: $VAR and ${VAR} from the
// environment, an unset one left as written.
func hermesExpandVars(s string) string {
	return os.Expand(s, func(name string) string {
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		if strings.ContainsAny(s, "{") && strings.Contains(s, "${"+name+"}") {
			return "${" + name + "}"
		}
		return "$" + name
	})
}

// hermesSkills fills an installed Hermes' skill dirs; ~/.agents/skills listed
// in external_dirs makes Hermes one more reader of the shared dir.
func (h *Hermes) hermesSkills(a *Agent) {
	home := h.configDir()
	a.ManagedDir = filepath.Join(home, "skills")
	a.ReadDirs = []string{a.ManagedDir}
	shared := filepath.Join(h.Home, ".agents", "skills")
	sharedReal, _ := filepath.EvalSymlinks(shared)
	for _, dir := range h.skillDirs(home, a.ManagedDir) {
		if sharedReal != "" && dir == sharedReal {
			a.SharedDir, dir = shared, shared
		}
		if !slices.Contains(a.ReadDirs, dir) {
			a.ReadDirs = append(a.ReadDirs, dir)
		}
	}
}
