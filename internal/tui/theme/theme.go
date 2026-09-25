// Package theme maps semantic roles (the SP Night schema: ui, syntax,
// diagnostic, git, ansi) to live color tokens. Bundled themes are embedded;
// user themes come from <ConfigDir>/themes via LoadUser.
package theme

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"sort"
	"sync"
	"sync/atomic"
)

const Default = "sp-night"

// spNight are the bundled SP Night flavors, always listed first.
var spNight = []string{"sp-night", "sp-night-garoa", "sp-night-jaragua"}

// legacyIDs maps the SP Night ids used up to v0.2 to the current ones, so a
// config.yaml theme or a user theme's extends written with an old id keeps
// working. The old ids stay reserved: no user theme can take them.
var legacyIDs = map[string]string{
	"noite":   "sp-night",
	"garoa":   "sp-night-garoa",
	"jaragua": "sp-night-jaragua",
}

// canonical returns the current id for a legacy one, and any other id as is.
func canonical(id string) string {
	if c, ok := legacyIDs[id]; ok {
		return c
	}
	return id
}

// Palette is a fully resolved theme: every role of the schema has a color.
type Palette struct {
	ID          string
	Label       string
	Description string
	Appearance  string // dark | light
	User        bool   // came from <ConfigDir>/themes, not from the binary
	// Named is the theme's own palette (name → hex), e.g. SP Night's 23 colors.
	Named map[string]string
	// Roles maps every schema role ("ui.accent", "ansi.red"…) to hex.
	Roles map[string]string
	// Colors maps each exported token (Primary, Bg, Info…) and each role path
	// to hex. It is what plugins receive in init.theme.colors.
	Colors map[string]string

	spec     map[string]string // role → "#hex" or palette name, before resolution
	resolved map[string]color.Color
}

var (
	mu       sync.RWMutex
	palettes = map[string]*Palette{}
	builtin  []string // ids in display order
	user     []string
	active   atomic.Pointer[Palette]
)

func init() {
	ps, err := loadBuiltin()
	if err != nil {
		panic(err) // a broken embedded theme is a build defect
	}
	for _, p := range ps {
		palettes[p.ID] = p
		builtin = append(builtin, p.ID)
	}
}

func Current() string {
	if p := active.Load(); p != nil {
		return p.ID
	}
	return Default
}

func Apply(id string) error {
	if id == "" {
		id = Default
	}
	p, ok := lookup(id)
	if !ok {
		return fmt.Errorf("unknown theme: %s", id)
	}
	active.Store(p)
	return nil
}

// Options lists themes in display order: SP Night, community themes (by id),
// then user themes (by id).
func Options() []Palette {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Palette, 0, len(builtin)+len(user))
	for _, id := range append(append([]string{}, builtin...), user...) {
		out = append(out, *palettes[id])
	}
	return out
}

func lookup(id string) (*Palette, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := palettes[canonical(id)]
	return p, ok
}

func orderBuiltin(ids []string) []string {
	rank := map[string]int{}
	for i, id := range spNight {
		rank[id] = i + 1
	}
	sort.Slice(ids, func(i, j int) bool {
		ri, rj := rank[ids[i]], rank[ids[j]]
		if ri != rj {
			if ri == 0 || rj == 0 {
				return ri != 0
			}
			return ri < rj
		}
		return ids[i] < ids[j]
	})
	return ids
}

// Tokens resolve at render time: existing Lip Gloss styles follow previews.
// Atomic swaps keep readers on an immutable palette, including async rendering.
// A token is either an exported name (Primary) or a role path (ansi.red).
type token string

func (t token) RGBA() (r, g, b, a uint32) {
	p := active.Load()
	if p == nil {
		p, _ = lookup(Default)
	}
	c, ok := p.resolved[string(t)]
	if !ok {
		return 0, 0, 0, 0xffff
	}
	return c.RGBA()
}

// tokenRoles is the single source of truth for exported tokens.
var tokenRoles = map[string]string{
	"Primary": "ui.accent", "Accent": "ui.accent_alt", "Subtle": "ui.fg_dim",
	"Bg": "ui.bg", "Deep": "ui.bg_deep", "Surface": "ui.panel", "Float": "ui.float",
	"Line": "ui.line", "Sel": "ui.selection", "Border": "ui.border",
	"BorderFocus": "ui.border_active", "Text": "ui.fg", "Bright": "ui.fg_bright",
	"Muted": "ui.fg_muted", "Cursor": "ui.cursor", "Link": "ui.link", "Match": "ui.match",
	"OnAccent": "ui.on_accent",

	"OK": "diagnostic.ok", "Warn": "diagnostic.warn", "Err": "diagnostic.error",
	"Info": "diagnostic.info", "Hint": "diagnostic.hint",

	"Added": "git.added", "Modified": "git.modified", "Removed": "git.removed",
	"Renamed": "git.renamed", "Staged": "git.staged", "Untracked": "git.untracked",
	"Conflict": "git.conflict",

	"SynKeyword": "syntax.keyword", "SynFunction": "syntax.function", "SynType": "syntax.type",
	"SynString": "syntax.string", "SynNumber": "syntax.number", "SynConstant": "syntax.constant",
	"SynComment": "syntax.comment", "SynTag": "syntax.tag", "SynPunct": "syntax.punctuation",
}

var (
	Primary     color.Color = token("Primary")
	Accent      color.Color = token("Accent")
	Subtle      color.Color = token("Subtle")
	Bg          color.Color = token("Bg")
	Deep        color.Color = token("Deep")
	Surface     color.Color = token("Surface")
	Float       color.Color = token("Float")
	Line        color.Color = token("Line")
	Text        color.Color = token("Text")
	Bright      color.Color = token("Bright")
	Muted       color.Color = token("Muted")
	Sel         color.Color = token("Sel")
	Border      color.Color = token("Border")
	BorderFocus color.Color = token("BorderFocus")
	Cursor      color.Color = token("Cursor")
	Link        color.Color = token("Link")
	Match       color.Color = token("Match")
	OnAccent    color.Color = token("OnAccent")

	OK   color.Color = token("OK")
	Warn color.Color = token("Warn")
	Err  color.Color = token("Err")
	Info color.Color = token("Info")
	Hint color.Color = token("Hint")

	Added     color.Color = token("Added")
	Modified  color.Color = token("Modified")
	Removed   color.Color = token("Removed")
	Renamed   color.Color = token("Renamed")
	Staged    color.Color = token("Staged")
	Untracked color.Color = token("Untracked")
	Conflict  color.Color = token("Conflict")

	SynKeyword  color.Color = token("SynKeyword")
	SynFunction color.Color = token("SynFunction")
	SynType     color.Color = token("SynType")
	SynString   color.Color = token("SynString")
	SynNumber   color.Color = token("SynNumber")
	SynConstant color.Color = token("SynConstant")
	SynComment  color.Color = token("SynComment")
	SynTag      color.Color = token("SynTag")
	SynPunct    color.Color = token("SynPunct")
)

// ANSI returns one of the 16 terminal colors of the active theme
// ("red", "bright_cyan"…).
func ANSI(name string) color.Color { return token("ansi." + name) }

func AgentColor(id string) color.Color {
	switch id {
	case "claude-code":
		return Primary
	case "codex":
		return Text
	case "gemini-cli":
		return Accent
	case "opencode":
		return OK
	}
	fallback := []color.Color{ANSI("magenta"), ANSI("cyan"), ANSI("blue"), ANSI("yellow"), ANSI("green")}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return fallback[int(h.Sum32())%len(fallback)]
}
