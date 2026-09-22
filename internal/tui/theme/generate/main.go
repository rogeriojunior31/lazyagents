// Command generate resolves canonical SP Night semantic roles into bundled themes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Hex values come exclusively from SP Night via semantic roles.
var mapping = map[string]string{
	"Primary": "ui.accent", "Accent": "ui.accent_alt", "Subtle": "ui.fg_dim",
	"Bg": "ui.bg", "Deep": "ui.bg_deep", "Surface": "ui.panel", "Text": "ui.fg",
	"Bright": "ui.fg_bright", "Muted": "ui.fg_muted", "Sel": "ui.selection",
	"Border": "ui.border", "BorderFocus": "ui.border_active", "OK": "diagnostic.ok",
	"Warn": "diagnostic.warn", "Err": "diagnostic.error",
}

func main() {
	source := flag.String("source", "", "SP-Night/sp-night directory")
	out := flag.String("out", "internal/tui/theme/themes", "output directory")
	check := flag.Bool("check", false, "verify generated files without writing")
	flag.Parse()
	if err := generate(*source, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(source, out string, check bool) error {
	var palette struct {
		Flavors map[string]struct {
			Label       string            `json:"label"`
			Description string            `json:"description"`
			Colors      map[string]string `json:"colors"`
		} `json:"flavors"`
	}
	read := func(name string, dst any) error {
		b, err := os.ReadFile(filepath.Join(source, "palette", name))
		if err != nil {
			return err
		}
		return json.Unmarshal(b, dst)
	}
	if err := read("sp_night.json", &palette); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := read("roles.json", &raw); err != nil {
		return err
	}
	roles := make(map[string]map[string]string)
	for _, group := range []string{"ui", "diagnostic"} {
		var values map[string]string
		if err := json.Unmarshal(raw[group], &values); err != nil {
			return err
		}
		roles[group] = values
	}
	for _, id := range []string{"noite", "garoa", "jaragua"} {
		flavor, ok := palette.Flavors[id]
		if !ok {
			return fmt.Errorf("missing SP Night flavor %s", id)
		}
		colors := make(map[string]string)
		for token, role := range mapping {
			parts := strings.Split(role, ".")
			value := flavor.Colors[roles[parts[0]][parts[1]]]
			if value == "" {
				return fmt.Errorf("unresolved role %s in %s", role, id)
			}
			colors[token] = value
		}
		data, err := json.MarshalIndent(struct {
			Source      string            `json:"source"`
			ID          string            `json:"id"`
			Label       string            `json:"label"`
			Description string            `json:"description"`
			Colors      map[string]string `json:"colors"`
		}{"Generated from SP Night palette + semantic roles; do not edit. https://github.com/sp-night/sp-night", id, flavor.Label, flavor.Description, colors}, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		path := filepath.Join(out, id+".json")
		if check {
			old, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if string(old) != string(data) {
				return fmt.Errorf("generated theme differs: %s", path)
			}
		} else if err := fsutil.WriteAtomic(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
