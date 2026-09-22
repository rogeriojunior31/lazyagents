// Package core concentra o estado de nível de aplicação compartilhado por todos
// os módulos: diretórios (XDG), config.yaml e helpers de exibição de paths.
// Não conhece agentes nem a TUI; fica na base da pilha (importa só fsutil).
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppName é o nome do app usado nos dirs XDG.
const AppName = "lazyagents"

// Paths concentra os diretórios do app; injetável em teste (PathsIn).
// Segue o padrão XDG: config em ~/.config, dados em ~/.local/share.
type Paths struct {
	Home            string // home do usuário
	ConfigDir       string // ~/.config/lazyagents        (config.yaml)
	DataDir         string // ~/.local/share/lazyagents    (skills, backups, profiles, exports)
	LibraryOverride string // override via config.yaml; vazio = default
}

// PathsIn monta Paths com o layout XDG padrão sob home, sem consultar o
// ambiente — base dos testes (home = t.TempDir()).
func PathsIn(home string) Paths {
	return Paths{
		Home:      home,
		ConfigDir: filepath.Join(home, ".config", AppName),
		DataDir:   filepath.Join(home, ".local", "share", AppName),
	}
}

// DefaultPaths resolve os dirs reais do usuário honrando $XDG_CONFIG_HOME e
// $XDG_DATA_HOME.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolvendo home: %w", err)
	}
	cfgHome, err := os.UserConfigDir() // honra $XDG_CONFIG_HOME; ~/.config no Linux
	if err != nil {
		return Paths{}, fmt.Errorf("resolvendo config dir: %w", err)
	}
	return Paths{
		Home:      home,
		ConfigDir: filepath.Join(cfgHome, AppName),
		DataDir:   filepath.Join(xdgDataHome(home), AppName),
	}, nil
}

// xdgDataHome resolve $XDG_DATA_HOME com fallback ~/.local/share.
func xdgDataHome(home string) string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".local", "share")
}

// LibraryDir é a biblioteca central de skills.
func (p Paths) LibraryDir() string {
	if p.LibraryOverride != "" {
		return p.LibraryOverride
	}
	return filepath.Join(p.DataDir, "skills")
}

func (p Paths) BackupsDir() string   { return filepath.Join(p.DataDir, "backups") }
func (p Paths) ExportsDir() string   { return filepath.Join(p.DataDir, "exports") }
func (p Paths) ProfilesPath() string { return filepath.Join(p.DataDir, "profiles.json") }
func (p Paths) ConfigPath() string   { return filepath.Join(p.ConfigDir, "config.yaml") }

// LegacyConfigPath é o config.json anterior ao yaml; só MigrateConfig o lê.
func (p Paths) LegacyConfigPath() string { return filepath.Join(p.ConfigDir, "config.json") }

// Tilde encurta o home para ~ na exibição.
func (p Paths) Tilde(path string) string { return Tilde(path, p.Home) }

// ExpandHome expande "~" e "~/..." para o home de p.
func (p Paths) ExpandHome(path string) string {
	switch {
	case path == "~":
		return p.Home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(p.Home, path[2:])
	}
	return path
}

// Tilde encurta home para ~ no início de path (exibição).
func Tilde(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
