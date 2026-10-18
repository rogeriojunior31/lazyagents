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

// Config retains the parsed document so Save preserves comments/foreign keys.
// ReadConfig again before merging edits made by another writer.
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

// Save writes changed theme/libraryDir fields into the original document;
// clearing a field removes its key. An optional original byte snapshot guards
// concurrent writes (nil means the file was absent).
func (c Config) Save(path string, before ...[]byte) error {
	root := c.root()
	for _, field := range []struct{ key, value string }{{"theme", c.Theme}, {"libraryDir", c.LibraryDir}} {
		previous := ""
		if n := get(root, field.key); n != nil && n.Kind == yaml.ScalarNode {
			previous = n.Value
		}
		if previous != field.value {
			set(root, field.key, field.value)
		}
	}
	data, err := encode(&c.doc)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}
	if len(before) > 0 {
		return fsutil.WriteAtomicIfUnchanged(path, before[0], before[0] != nil, data, 0o600)
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

// SetSectionValue changes one module-owned field without replacing its other
// keys or comments. Save persists the resulting document.
func (c *Config) SetSectionValue(id, key string, value any) error {
	if id == "" || id == "theme" || id == "libraryDir" || key == "" {
		return errors.New("invalid module config key")
	}
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return fmt.Errorf("encoding config section %s: %w", id, err)
	}
	root := c.root()
	section := get(root, id)
	if section == nil {
		section = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id}, section)
	}
	if section.Kind != yaml.MappingNode {
		return fmt.Errorf("config section %s must be a key: value map", id)
	}
	if existing := get(section, key); existing != nil {
		node.HeadComment, node.LineComment, node.FootComment = existing.HeadComment, existing.LineComment, existing.FootComment
		if node.Kind == existing.Kind {
			node.Style = existing.Style
		}
		*existing = node
	} else {
		section.Content = append(section.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &node)
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
