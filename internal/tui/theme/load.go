package theme

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"
)

//go:embed themes/*.yaml
var files embed.FS

// Schema is the SP Night role layer (palette/roles.json). Every theme defines
// all of these, directly or through extends.
var Schema = []struct {
	Group string
	Roles []string
}{
	{"ui", []string{"bg", "bg_deep", "panel", "float", "line", "selection", "border", "border_active",
		"fg_bright", "fg", "fg_dim", "fg_muted", "cursor", "accent", "accent_alt", "link", "match", "on_accent"}},
	{"syntax", []string{"keyword", "conditional", "repeat", "operator", "function", "method", "constructor",
		"type", "namespace", "macro", "parameter", "constant", "number", "boolean", "builtin", "attribute",
		"string", "character", "escape", "variable", "property", "field", "punctuation", "comment", "tag", "deprecated"}},
	{"diagnostic", []string{"error", "warn", "info", "hint", "ok"}},
	{"git", []string{"added", "modified", "removed", "renamed", "staged", "untracked", "conflict"}},
	{"ansi", []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"bright_black", "bright_red", "bright_green", "bright_yellow", "bright_blue", "bright_magenta",
		"bright_cyan", "bright_white"}},
}

// file is the on-disk format, shared by bundled and user themes.
type file struct {
	ID          string            `yaml:"id"`
	Label       string            `yaml:"label"`
	Description string            `yaml:"description"`
	Appearance  string            `yaml:"appearance"`
	Extends     string            `yaml:"extends"`
	Palette     map[string]string `yaml:"palette"`
	UI          map[string]string `yaml:"ui"`
	Syntax      map[string]string `yaml:"syntax"`
	Diagnostic  map[string]string `yaml:"diagnostic"`
	Git         map[string]string `yaml:"git"`
	ANSI        map[string]string `yaml:"ansi"`
}

func (f *file) groups() map[string]map[string]string {
	return map[string]map[string]string{
		"ui": f.UI, "syntax": f.Syntax, "diagnostic": f.Diagnostic, "git": f.Git, "ansi": f.ANSI,
	}
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func parse(data []byte) (*file, error) {
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return &f, nil
}

func checkHex(where, v string) (string, error) {
	if v == "" {
		return "", fmt.Errorf("%s: empty color (hex needs quotes: \"#rrggbb\")", where)
	}
	if !hexRe.MatchString(v) {
		return "", fmt.Errorf("%s: invalid color %q (use \"#rrggbb\")", where, v)
	}
	return strings.ToLower(v), nil
}

// resolve builds a palette from f on top of base (nil for a standalone theme).
// Roles inherited from base keep their palette references, so overriding a
// named color in the child recolors every role that uses it.
func resolve(id string, f *file, base *Palette) (*Palette, error) {
	p := &Palette{ID: id, Label: f.Label, Description: f.Description, Appearance: f.Appearance,
		Named: map[string]string{}, spec: map[string]string{}}
	if p.Label == "" {
		p.Label = id
	}
	if base != nil {
		for k, v := range base.Named {
			p.Named[k] = v
		}
		for k, v := range base.spec {
			p.spec[k] = v
		}
		if p.Appearance == "" {
			p.Appearance = base.Appearance
		}
	}
	if p.Appearance == "" {
		p.Appearance = "dark"
	}
	if p.Appearance != "dark" && p.Appearance != "light" {
		return nil, fmt.Errorf("appearance %q: use dark or light", p.Appearance)
	}
	for name, v := range f.Palette {
		hex, err := checkHex("palette."+name, v)
		if err != nil {
			return nil, err
		}
		p.Named[name] = hex
	}
	known := map[string]bool{}
	for _, g := range Schema {
		for _, r := range g.Roles {
			known[g.Group+"."+r] = true
		}
	}
	for group, roles := range f.groups() {
		for r, v := range roles {
			key := group + "." + r
			if !known[key] {
				return nil, fmt.Errorf("unknown role %s", key)
			}
			p.spec[key] = v
		}
	}
	p.Roles = map[string]string{}
	for _, g := range Schema {
		for _, r := range g.Roles {
			key := g.Group + "." + r
			v, ok := p.spec[key]
			if !ok {
				return nil, fmt.Errorf("missing role %s", key)
			}
			if !strings.HasPrefix(v, "#") && v != "" {
				named, ok := p.Named[v]
				if !ok {
					return nil, fmt.Errorf("%s: color %q is not in palette", key, v)
				}
				v = named
			}
			hex, err := checkHex(key, v)
			if err != nil {
				return nil, err
			}
			p.Roles[key] = hex
		}
	}
	p.Colors = map[string]string{}
	p.resolved = map[string]color.Color{}
	for key, hex := range p.Roles {
		p.Colors[key] = hex
		p.resolved[key] = lipgloss.Color(hex)
	}
	for name, role := range tokenRoles {
		p.Colors[name] = p.Roles[role]
		p.resolved[name] = p.resolved[role]
	}
	return p, nil
}

func loadBuiltin() ([]*Palette, error) {
	names, err := files.ReadDir("themes")
	if err != nil {
		return nil, err
	}
	var ids []string
	out := map[string]*Palette{}
	for _, e := range names {
		id := strings.TrimSuffix(e.Name(), ".yaml")
		data, err := files.ReadFile("themes/" + e.Name())
		if err != nil {
			return nil, err
		}
		f, err := parse(data)
		if err != nil {
			return nil, fmt.Errorf("built-in theme %s: %w", id, err)
		}
		if f.ID != id || f.Extends != "" {
			return nil, fmt.Errorf("built-in theme %s: id must match the file name and extends is not allowed", id)
		}
		p, err := resolve(id, f, nil)
		if err != nil {
			return nil, fmt.Errorf("built-in theme %s: %w", id, err)
		}
		out[id] = p
		ids = append(ids, id)
	}
	var ps []*Palette
	for _, id := range orderBuiltin(ids) {
		ps = append(ps, out[id])
	}
	return ps, nil
}

// LoadUser registers the themes in dir (<id>.yaml or <id>.yml), replacing
// those of a previous call. A theme without extends builds on top of Default.
// A broken file is skipped and reported; the others still load. A missing dir
// is not an error.
func LoadUser(dir string) []error {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return []error{fmt.Errorf("reading themes in %s: %w", dir, err)}
	}
	var errs []error
	parsed := map[string]*file{}
	paths := map[string]string{}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ext)
		path := filepath.Join(dir, e.Name())
		fail := func(err error) { errs = append(errs, fmt.Errorf("theme %s: %w", path, err)) }
		if _, ok := lookupBuiltin(id); ok {
			fail(fmt.Errorf("%q is a built-in theme; use another name with extends: %s", id, canonical(id)))
			continue
		}
		if _, dup := parsed[id]; dup {
			fail(fmt.Errorf("duplicate id %q", id))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fail(err)
			continue
		}
		f, err := parse(data)
		if err != nil {
			fail(err)
			continue
		}
		if f.ID != "" && f.ID != id {
			fail(fmt.Errorf("id %q differs from the file name (%s)", f.ID, id))
			continue
		}
		parsed[id] = f
		paths[id] = path
	}

	done := map[string]*Palette{}
	failed := map[string]bool{}
	visiting := map[string]bool{}
	var get func(id string) (*Palette, error)
	get = func(id string) (*Palette, error) {
		if p, ok := lookupBuiltin(id); ok {
			return p, nil
		}
		if p, ok := done[id]; ok {
			return p, nil
		}
		f, ok := parsed[id]
		if !ok || failed[id] {
			return nil, fmt.Errorf("theme %q does not exist or is invalid", id)
		}
		if visiting[id] {
			return nil, fmt.Errorf("extends cycle through %q", id)
		}
		visiting[id] = true
		defer delete(visiting, id)
		parent := f.Extends
		if parent == "" {
			parent = Default
		}
		base, err := get(parent)
		if err != nil {
			return nil, fmt.Errorf("extends %s: %w", parent, err)
		}
		p, err := resolve(id, f, base)
		if err != nil {
			return nil, err
		}
		p.User = true
		done[id] = p
		return p, nil
	}
	ids := make([]string, 0, len(parsed))
	for id := range parsed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := get(id); err != nil {
			failed[id] = true
			errs = append(errs, fmt.Errorf("theme %s: %w", paths[id], err))
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, id := range user {
		delete(palettes, id)
	}
	user = user[:0]
	for _, id := range ids {
		if p, ok := done[id]; ok {
			palettes[id] = p
			user = append(user, id)
		}
	}
	return errs
}

func lookupBuiltin(id string) (*Palette, bool) {
	id = canonical(id)
	mu.RLock()
	defer mu.RUnlock()
	for _, b := range builtin {
		if b == id {
			return palettes[id], true
		}
	}
	return nil, false
}
