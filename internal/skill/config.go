package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"lazyskills/internal/fsutil"
)

// config é a estrutura tipada da config.json. Campos desconhecidos são
// preservados no round-trip via readConfigRaw/saveConfig.
type config struct {
	LibraryDir string `json:"libraryDir,omitempty"`
}

// configPath retorna o caminho do arquivo de config.
func configPath(dataDir string) string {
	return filepath.Join(dataDir, "config.json")
}

// readConfigRaw lê config.json preservando campos desconhecidos.
// Arquivo ausente é OK — retorna zero values sem erro.
func readConfigRaw(path string) (map[string]json.RawMessage, config, error) {
	raw := make(map[string]json.RawMessage)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return raw, config{}, nil
		}
		return nil, config{}, fmt.Errorf("lendo config: %w", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, config{}, fmt.Errorf("parseando config: %w", err)
	}
	var cfg config
	if v, ok := raw["libraryDir"]; ok {
		_ = json.Unmarshal(v, &cfg.LibraryDir)
	}
	return raw, cfg, nil
}

// saveConfig persiste a config mesclando cfg sobre os campos brutos existentes.
func saveConfig(path string, raw map[string]json.RawMessage, cfg config) error {
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	if cfg.LibraryDir != "" {
		v, _ := json.Marshal(cfg.LibraryDir)
		raw["libraryDir"] = v
	} else {
		delete(raw, "libraryDir")
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, data, 0o600)
}

// LoadPaths é como DefaultPaths, mas lê config.json para honrar overrides
// (ex.: libraryDir personalizado). Usa DefaultPaths se o arquivo não existir.
func LoadPaths() (Paths, error) {
	p, err := DefaultPaths()
	if err != nil {
		return Paths{}, err
	}
	_, cfg, err := readConfigRaw(configPath(p.DataDir))
	if err != nil {
		return p, nil // config inválida → ignora silenciosamente
	}
	if cfg.LibraryDir != "" {
		dir := cfg.LibraryDir
		if len(dir) >= 2 && dir[:2] == "~/" {
			dir = filepath.Join(p.Home, dir[2:])
		} else if dir == "~" {
			dir = p.Home
		}
		p.LibraryOverride = dir
	}
	return p, nil
}
