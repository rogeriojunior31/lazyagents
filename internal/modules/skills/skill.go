// Package skills gerencia a biblioteca central de skills
// (~/.local/share/lazyagents/skills) e a ativação delas por agente via symlink
// no dir de skills de cada um.
package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Meta é o frontmatter YAML de um SKILL.md.
type Meta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// AgentState é o estado de uma skill em UM agente.
type AgentState struct {
	On      bool   // visível para o agente
	Managed bool   // symlink do lazyagents no ManagedDir → Enable/Disable funciona
	Local   bool   // conteúdo real (ou symlink alheio) que o lazyagents não controla
	Via     string // dir onde a skill foi encontrada
}

// Skill é uma skill unificada: presente na biblioteca e/ou nos agentes.
type Skill struct {
	Dir         string // nome canônico da pasta (chave de todas as operações)
	Name        string // frontmatter name, fallback Dir
	Description string
	Path        string  // pasta na biblioteca (ou origem local, se fora dela)
	Origin      *Origin // proveniência (.origin.json); nil = criada manualmente
	InLibrary   bool
	Valid       bool
	Warning     string
	States      map[string]AgentState // agentID → estado
}

// EnabledCount conta em quantos agentes a skill está visível.
func (s Skill) EnabledCount() int {
	n := 0
	for _, st := range s.States {
		if st.On {
			n++
		}
	}
	return n
}

// Service executa scan e operações de skills. Não conhece a TUI.
type Service struct {
	paths core.Paths
}

func New(paths core.Paths) *Service { return &Service{paths: paths} }

func (s *Service) Paths() core.Paths { return s.paths }

// Scan varre a biblioteca e todos os dirs de skills lidos por cada agente
// instalado, unificando por nome de pasta. Nunca falha por skill quebrada:
// frontmatter inválido vira Valid=false + Warning.
func (s *Service) Scan(agents []agent.Agent) ([]Skill, error) {
	byDir := make(map[string]*Skill)

	// 1) biblioteca central
	libDir := s.paths.LibraryDir()
	entries, err := os.ReadDir(libDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("lendo biblioteca %s: %w", libDir, err)
	}
	for _, e := range entries {
		path := filepath.Join(libDir, e.Name())
		if !dirWithSkillMD(path) {
			continue
		}
		sk := parseSkill(path, e.Name())
		sk.InLibrary = true
		byDir[e.Name()] = &sk
	}

	// 2) dirs de cada agente
	for _, ag := range agents {
		if !ag.Installed {
			continue
		}
		for _, dir := range ag.ReadDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				path := filepath.Join(dir, e.Name())
				if !dirWithSkillMD(path) {
					continue
				}
				sk, ok := byDir[e.Name()]
				if !ok {
					parsed := parseSkill(path, e.Name())
					sk = &parsed
					byDir[e.Name()] = sk
				}
				st := sk.States[ag.ID]
				if st.On {
					continue // já encontrada num dir de maior prioridade
				}
				st = AgentState{On: true, Via: dir}
				if dir == libDir {
					// skill da biblioteca lida diretamente pelo agente: não local
					st.Managed = dir == ag.ManagedDir
				} else if target, err := os.Readlink(path); err == nil {
					resolved := target
					if !filepath.IsAbs(resolved) {
						resolved = filepath.Join(dir, target)
					}
					if insideDir(resolved, libDir) {
						st.Managed = dir == ag.ManagedDir
					} else {
						st.Local = true // symlink alheio (ex.: omarchy)
					}
				} else {
					st.Local = true // diretório real
				}
				sk.States[ag.ID] = st
			}
		}
	}

	out := make([]Skill, 0, len(byDir))
	for _, sk := range byDir {
		out = append(out, *sk)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func dirWithSkillMD(path string) bool {
	info, err := os.Stat(path) // segue symlink de propósito
	if err != nil || !info.IsDir() {
		return false
	}
	f, err := os.Stat(filepath.Join(path, "SKILL.md"))
	return err == nil && f.Mode().IsRegular()
}

func insideDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func parseSkill(path, dirName string) Skill {
	sk := Skill{
		Dir:    dirName,
		Name:   dirName,
		Path:   path,
		Origin: readOrigin(path),
		States: make(map[string]AgentState),
	}
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		sk.Warning = fmt.Sprintf("lendo SKILL.md: %v", err)
		return sk
	}
	meta, ok := ParseMeta(data)
	if !ok {
		sk.Warning = "SKILL.md has no valid YAML frontmatter"
		return sk
	}
	sk.Valid = true
	if meta.Name != "" {
		sk.Name = meta.Name
	}
	sk.Description = meta.Description
	return sk
}

// ParseMeta extrai o frontmatter YAML (entre cercas ---) de um SKILL.md.
func ParseMeta(data []byte) (Meta, bool) {
	fm, ok := frontmatter(data)
	if !ok {
		return Meta{}, false
	}
	var m Meta
	if err := yaml.Unmarshal(fm, &m); err != nil {
		return Meta{}, false
	}
	return m, true
}

func frontmatter(data []byte) ([]byte, bool) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // BOM UTF-8
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || string(bytes.TrimRight(lines[0], "\r ")) != "---" {
		return nil, false
	}
	for i := 1; i < len(lines); i++ {
		t := string(bytes.TrimRight(lines[i], "\r "))
		if t == "---" || t == "..." {
			return bytes.Join(lines[1:i], []byte("\n")), true
		}
	}
	return nil, false
}
