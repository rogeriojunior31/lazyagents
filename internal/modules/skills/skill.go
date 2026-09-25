// Package skills manages the central skills library
// (~/.local/share/lazyagents/skills) and enables skills per agent by symlink
// into each agent's skills dir.
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

// Meta is a SKILL.md's YAML frontmatter.
type Meta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// AgentState is a skill's state in ONE agent.
type AgentState struct {
	On      bool   // visible to the agent
	Managed bool   // lazyagents symlink in ManagedDir → Enable/Disable work
	Local   bool   // real content (or foreign symlink) lazyagents does not control
	Via     string // dir where the skill was found
}

// Skill is a unified skill: in the library and/or in agents.
type Skill struct {
	Dir         string // canonical folder name (key of every operation)
	Name        string // frontmatter name, else Dir
	Description string
	Path        string  // library folder (or local source, if outside it)
	Origin      *Origin // provenance (.origin.json); nil = created by hand
	InLibrary   bool
	Valid       bool
	Warning     string
	States      map[string]AgentState // agentID → state
}

// EnabledCount counts the agents where the skill is visible.
func (s Skill) EnabledCount() int {
	n := 0
	for _, st := range s.States {
		if st.On {
			n++
		}
	}
	return n
}

// Service runs skill scans and operations. It does not know the TUI.
type Service struct {
	paths core.Paths
}

func New(paths core.Paths) *Service { return &Service{paths: paths} }

func (s *Service) Paths() core.Paths { return s.paths }

// Scan walks the library and every skills dir read by each installed agent,
// merging by folder name. Never fails on a broken skill: invalid frontmatter
// becomes Valid=false + Warning.
func (s *Service) Scan(agents []agent.Agent) ([]Skill, error) {
	byDir := make(map[string]*Skill)

	// 1) central library
	libDir := s.paths.LibraryDir()
	entries, err := os.ReadDir(libDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading library %s: %w", libDir, err)
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

	// 2) each agent's dirs
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
					continue // already found in a higher-priority dir
				}
				st = AgentState{On: true, Via: dir}
				if dir == libDir {
					// library skill read directly by the agent: not local
					st.Managed = dir == ag.ManagedDir
				} else if target, err := os.Readlink(path); err == nil {
					resolved := target
					if !filepath.IsAbs(resolved) {
						resolved = filepath.Join(dir, target)
					}
					if insideDir(resolved, libDir) {
						st.Managed = dir == ag.ManagedDir
					} else {
						st.Local = true // foreign symlink (e.g. omarchy)
					}
				} else {
					st.Local = true // real directory
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
	info, err := os.Stat(path) // follows symlinks on purpose
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
		sk.Warning = fmt.Sprintf("reading SKILL.md: %v", err)
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

// ParseMeta extracts the YAML frontmatter (between --- fences) of a SKILL.md.
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
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM
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
