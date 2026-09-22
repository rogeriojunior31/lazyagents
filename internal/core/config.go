package core

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Config é a estrutura tipada da config.json. Campos desconhecidos são
// preservados no round-trip via ReadConfigRaw/SaveConfig.
type Config struct {
	LibraryDir string `json:"libraryDir,omitempty"`
	Theme      string `json:"theme,omitempty"`
}

// ReadConfigRaw lê config.json preservando campos desconhecidos.
// Arquivo ausente é OK — retorna zero values sem erro.
func ReadConfigRaw(path string) (map[string]json.RawMessage, Config, error) {
	raw := make(map[string]json.RawMessage)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return raw, Config{}, nil
		}
		return nil, Config{}, fmt.Errorf("lendo config: %w", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, Config{}, fmt.Errorf("parseando config: %w", err)
	}
	var cfg Config
	if v, ok := raw["libraryDir"]; ok {
		_ = json.Unmarshal(v, &cfg.LibraryDir)
	}
	if v, ok := raw["theme"]; ok {
		_ = json.Unmarshal(v, &cfg.Theme)
	}
	return raw, cfg, nil
}

// SaveConfig persiste a config mesclando cfg sobre os campos brutos existentes.
func SaveConfig(path string, raw map[string]json.RawMessage, cfg Config) error {
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	if cfg.LibraryDir != "" {
		v, _ := json.Marshal(cfg.LibraryDir)
		raw["libraryDir"] = v
	} else {
		delete(raw, "libraryDir")
	}
	if cfg.Theme != "" {
		v, _ := json.Marshal(cfg.Theme)
		raw["theme"] = v
	} else {
		delete(raw, "theme")
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, data, 0o600)
}

// LoadPaths é como DefaultPaths, mas lê config.json para honrar overrides
// (ex.: libraryDir personalizado).
func LoadPaths() (Paths, error) {
	p, err := DefaultPaths()
	if err != nil {
		return Paths{}, err
	}
	return p.WithConfig(), nil
}

// WithConfig aplica os overrides de config.json sobre p. Config inválida é
// ignorada (comportamento histórico: nunca trava o boot).
func (p Paths) WithConfig() Paths {
	_, cfg, err := ReadConfigRaw(p.ConfigPath())
	if err != nil {
		return p
	}
	if cfg.LibraryDir != "" {
		p.LibraryOverride = p.ExpandHome(cfg.LibraryDir)
	}
	return p
}
