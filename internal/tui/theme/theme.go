// Package theme maps SP Night semantic roles to live color tokens.
package theme

import (
	"embed"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image/color"
	"sync/atomic"

	"charm.land/lipgloss/v2"
)

//go:embed themes/*.json
var files embed.FS

const Default = "noite"

type Palette struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Colors      map[string]string `json:"colors"`
	resolved    map[string]color.Color
}

var palettes = load()
var active atomic.Pointer[Palette]

func load() map[string]*Palette {
	result := make(map[string]*Palette)
	for _, id := range []string{"noite", "garoa", "jaragua"} {
		data, err := files.ReadFile("themes/" + id + ".json")
		if err != nil {
			panic(err)
		} // A missing embedded asset is a build defect.
		var p Palette
		if err := json.Unmarshal(data, &p); err != nil {
			panic(err)
		}
		p.resolved = make(map[string]color.Color)
		for role, hex := range p.Colors {
			p.resolved[role] = lipgloss.Color(hex)
		}
		result[id] = &p
	}
	return result
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
	p, ok := palettes[id]
	if !ok {
		return fmt.Errorf("tema desconhecido: %s", id)
	}
	active.Store(p)
	return nil
}

func Options() []Palette {
	return []Palette{*palettes["noite"], *palettes["garoa"], *palettes["jaragua"]}
}

// Tokens resolve at render time: existing Lip Gloss styles follow previews.
// Atomic swaps keep readers on an immutable palette, including async rendering.
type token string

func (t token) RGBA() (r, g, b, a uint32) {
	p := active.Load()
	if p == nil {
		p = palettes[Default]
	}
	return p.resolved[string(t)].RGBA()
}

var (
	Primary     color.Color = token("Primary")
	Accent      color.Color = token("Accent")
	Subtle      color.Color = token("Subtle")
	Bg          color.Color = token("Bg")
	Deep        color.Color = token("Deep")
	Surface     color.Color = token("Surface")
	Text        color.Color = token("Text")
	Bright      color.Color = token("Bright")
	Muted       color.Color = token("Muted")
	Sel         color.Color = token("Sel")
	Border      color.Color = token("Border")
	BorderFocus color.Color = token("BorderFocus")
	OK          color.Color = token("OK")
	Warn        color.Color = token("Warn")
	Err         color.Color = token("Err")
)

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
	fallback := []color.Color{Accent, Primary, OK, Warn, Subtle}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return fallback[int(h.Sum32())%len(fallback)]
}
