// Package core holds app-level state shared by every module: XDG dirs,
// config.yaml and path display helpers. Bottom of the stack: imports only fsutil.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppName names the XDG dirs.
const AppName = "lazyagents"

// Paths holds the app dirs (XDG layout); tests inject it with PathsIn.
type Paths struct {
	Home            string
	ConfigDir       string // ~/.config/lazyagents        (config.yaml)
	DataDir         string // ~/.local/share/lazyagents    (skills, backups, profiles, exports)
	LibraryOverride string // from config.yaml; empty = default
}

// PathsIn builds the default XDG layout under home without reading the
// environment, for tests.
func PathsIn(home string) Paths {
	return Paths{
		Home:      home,
		ConfigDir: filepath.Join(home, ".config", AppName),
		DataDir:   filepath.Join(home, ".local", "share", AppName),
	}
}

// DefaultPaths resolves the user's real dirs, honoring $XDG_CONFIG_HOME and
// $XDG_DATA_HOME.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolving home: %w", err)
	}
	cfgHome, err := os.UserConfigDir() // honors $XDG_CONFIG_HOME
	if err != nil {
		return Paths{}, fmt.Errorf("resolving config dir: %w", err)
	}
	return Paths{
		Home:      home,
		ConfigDir: filepath.Join(cfgHome, AppName),
		DataDir:   filepath.Join(xdgDataHome(home), AppName),
	}, nil
}

func xdgDataHome(home string) string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".local", "share")
}

// LibraryDir is the central skills library.
func (p Paths) LibraryDir() string {
	if p.LibraryOverride != "" {
		return p.LibraryOverride
	}
	return filepath.Join(p.DataDir, "skills")
}

func (p Paths) BackupsDir() string   { return filepath.Join(p.DataDir, "backups") }
func (p Paths) ExportsDir() string   { return filepath.Join(p.DataDir, "exports") }
func (p Paths) ProfilesPath() string { return filepath.Join(p.DataDir, "profiles.json") }

// HooksDir holds one hand-editable JSON file per hook.
func (p Paths) HooksDir() string    { return filepath.Join(p.DataDir, "hooks") }
func (p Paths) AliasesPath() string { return filepath.Join(p.DataDir, "session-aliases.json") }

// UsageCachePath caches subscription limits (with a TTL) so opening the tab
// does not hit the network every time.
func (p Paths) UsageCachePath() string { return filepath.Join(p.DataDir, "usage-cache.json") }

// TranscriptIndexPath is a disposable cache: deleting it only makes the next
// load reread every transcript.
func (p Paths) TranscriptIndexPath() string {
	return filepath.Join(p.DataDir, "transcript-index.gob")
}
func (p Paths) ConfigPath() string { return filepath.Join(p.ConfigDir, "config.yaml") }

// ProvidersPath lives in ConfigDir because it is user config; the file is
// 0600 since it may hold tokens.
func (p Paths) ProvidersPath() string { return filepath.Join(p.ConfigDir, "providers.json") }
func (p Paths) PluginsDir() string    { return filepath.Join(p.ConfigDir, "plugins") }

// ThemesDir holds user themes (<id>.yaml).
func (p Paths) ThemesDir() string { return filepath.Join(p.ConfigDir, "themes") }

// LegacyConfigPath is the pre-yaml config.json; only MigrateConfig reads it.
func (p Paths) LegacyConfigPath() string { return filepath.Join(p.ConfigDir, "config.json") }

// Tilde shortens p.Home to ~ for display.
func (p Paths) Tilde(path string) string { return Tilde(path, p.Home) }

// ExpandHome expands "~" and "~/..." to p.Home.
func (p Paths) ExpandHome(path string) string {
	switch {
	case path == "~":
		return p.Home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(p.Home, path[2:])
	}
	return path
}

// Tilde shortens a leading home to ~ for display.
func Tilde(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
