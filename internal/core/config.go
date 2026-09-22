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

// Config é a visão tipada do config.yaml mais o documento parseado, que mantém
// chaves desconhecidas e comentários no Save. Só nasce via ReadConfig, então
// todo Save é um read-modify-write do arquivo vivo.
//
// Chaves reservadas: theme e libraryDir. Qualquer outra chave de topo é a
// seção do módulo/plugin de mesmo id, lida sob demanda com Section.
type Config struct {
	Theme      string
	LibraryDir string
	doc        yaml.Node // DocumentNode; zero quando o arquivo não existe
}

// ReadConfig lê config.yaml. Arquivo ausente é OK — devolve zero values sem
// erro. YAML inválido ou raiz que não é um mapa é erro.
func ReadConfig(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, fmt.Errorf("lendo config: %w", err)
	}
	if err := yaml.Unmarshal(data, &c.doc); err != nil {
		return Config{}, fmt.Errorf("parseando config: %w", err)
	}
	if len(c.doc.Content) > 0 && c.doc.Content[0].Kind != yaml.MappingNode {
		return Config{}, errors.New("parseando config: raiz precisa ser um mapa chave: valor")
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

// Save persiste theme/libraryDir sobre o documento original: campo vazio
// remove a chave; comentários e chaves alheias ficam como estavam.
func (c Config) Save(path string) error {
	root := c.root()
	set(root, "theme", c.Theme)
	set(root, "libraryDir", c.LibraryDir)
	data, err := encode(&c.doc)
	if err != nil {
		return fmt.Errorf("serializando config: %w", err)
	}
	return fsutil.WriteAtomic(path, data, 0o600)
}

// Section decodifica a chave de topo id em out. Chave ausente não é erro e
// deixa out intacto.
func (c Config) Section(id string, out any) error {
	if len(c.doc.Content) == 0 {
		return nil
	}
	n := get(c.doc.Content[0], id)
	if n == nil {
		return nil
	}
	if err := n.Decode(out); err != nil {
		return fmt.Errorf("seção %s da config: %w", id, err)
	}
	return nil
}

// MigrateConfig converte um config.json legado em config.yaml (mesmas chaves,
// nada perdido) e renomeia o original para config.json.migrated. Idempotente:
// com o yaml já presente, ou sem json, não faz nada.
func MigrateConfig(p Paths) (bool, error) {
	if _, err := os.Stat(p.ConfigPath()); err == nil {
		return false, nil
	}
	data, err := os.ReadFile(p.LegacyConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("lendo config.json: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return false, fmt.Errorf("config.json inválido, migre para config.yaml manualmente: %w", err)
	}
	var doc yaml.Node
	if err := doc.Encode(m); err != nil {
		return false, fmt.Errorf("convertendo config.json: %w", err)
	}
	out, err := encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{&doc}})
	if err != nil {
		return false, fmt.Errorf("convertendo config.json: %w", err)
	}
	if err := fsutil.WriteAtomic(p.ConfigPath(), out, 0o600); err != nil {
		return false, err
	}
	// rename não é escrita de conteúdo; o yaml já está seguro em disco.
	if err := os.Rename(p.LegacyConfigPath(), p.LegacyConfigPath()+".migrated"); err != nil {
		return true, fmt.Errorf("renomeando config.json: %w", err)
	}
	return true, nil
}

// WithConfig aplica os overrides de config.yaml sobre p. Config inválida é
// ignorada (nunca trava o boot).
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

// root devolve o mapping raiz, criando o documento quando o arquivo não existia.
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

// get devolve o nó-valor da chave em m (mapping), ou nil.
func get(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// set grava um escalar: muta o nó existente in place (o comentário de linha
// sobrevive), apende se não existe, remove o par quando value é vazio.
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
