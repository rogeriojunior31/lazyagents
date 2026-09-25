package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Config is the typed view of config.yaml plus the parsed document, which
// keeps unknown keys and comments on Save. Built only by ReadConfig, so every
// Save is a read-modify-write of the live file.
//
// Reserved keys: theme and libraryDir. Any other top-level key is the section
// of the module or plugin with that id, read on demand with Section.
type Config struct {
	Theme      string
	LibraryDir string
	doc        yaml.Node // DocumentNode; zero when the file does not exist
}

// ReadConfig reads config.yaml. A missing file yields zero values, not an error.
func ReadConfig(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, fmt.Errorf("reading config: %w", err)
	}
	if err := yaml.Unmarshal(data, &c.doc); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	if len(c.doc.Content) > 0 && c.doc.Content[0].Kind != yaml.MappingNode {
		return Config{}, errors.New("parsing config: the root must be a key: value map")
	}
	root := c.root()
	if n := get(root, "theme"); n != nil && n.Kind == yaml.ScalarNode {
		c.Theme = n.Value
	}
	if n := get(root, "libraryDir"); n != nil && n.Kind == yaml.ScalarNode {
		c.LibraryDir = n.Value
	}
	return c, nil
}

// Save writes theme and libraryDir into the original document; an empty field
// removes its key, and comments and foreign keys are kept.
func (c Config) Save(path string) error {
	root := c.root()
	set(root, "theme", c.Theme)
	set(root, "libraryDir", c.LibraryDir)
	data, err := encode(&c.doc)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}
	return fsutil.WriteAtomic(path, data, 0o600)
}

// Section decodes the top-level key id into out. A missing key leaves out as is.
func (c Config) Section(id string, out any) error {
	if len(c.doc.Content) == 0 {
		return nil
	}
	n := get(c.doc.Content[0], id)
	if n == nil {
		return nil
	}
	if err := n.Decode(out); err != nil {
		return fmt.Errorf("config section %s: %w", id, err)
	}
	return nil
}

// MigrateConfig converts a legacy config.json into config.yaml (same keys) and
// renames the original to config.json.migrated. A no-op once the yaml exists.
func MigrateConfig(p Paths) (bool, error) {
	if _, err := os.Stat(p.ConfigPath()); err == nil {
		return false, nil
	}
	data, err := os.ReadFile(p.LegacyConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading config.json: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return false, fmt.Errorf("invalid config.json, migrate to config.yaml by hand: %w", err)
	}
	var doc yaml.Node
	if err := doc.Encode(m); err != nil {
		return false, fmt.Errorf("converting config.json: %w", err)
	}
	out, err := encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{&doc}})
	if err != nil {
		return false, fmt.Errorf("converting config.json: %w", err)
	}
	if err := fsutil.WriteAtomic(p.ConfigPath(), out, 0o600); err != nil {
		return false, err
	}
	// The yaml is already safe on disk; the rename is not a content write.
	if err := os.Rename(p.LegacyConfigPath(), p.LegacyConfigPath()+".migrated"); err != nil {
		return true, fmt.Errorf("renaming config.json: %w", err)
	}
	return true, nil
}

// WithConfig applies config.yaml overrides to p. An invalid config is ignored
// so it never blocks boot.
func (p Paths) WithConfig() Paths {
	cfg, err := ReadConfig(p.ConfigPath())
	if err != nil {
		return p
	}
	if cfg.LibraryDir != "" {
		p.LibraryOverride = p.ExpandHome(cfg.LibraryDir)
	}
	return p
}

// root returns the root mapping, creating the document for a missing file.
func (c *Config) root() *yaml.Node {
	if len(c.doc.Content) == 0 {
		c.doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	return c.doc.Content[0]
}

func encode(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func get(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// set updates the existing node in place so its line comment survives; an
// empty value removes the pair.
func set(m *yaml.Node, key, value string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != key {
			continue
		}
		if value == "" {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
		v := m.Content[i+1]
		v.Kind, v.Tag, v.Value, v.Content = yaml.ScalarNode, "!!str", value, nil
		return
	}
	if value == "" {
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}
