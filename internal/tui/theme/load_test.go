package theme

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltinThemesAreComplete(t *testing.T) {
	var ids []string
	for _, p := range Options() {
		ids = append(ids, p.ID)
		for _, g := range Schema {
			for _, r := range g.Roles {
				if !hexRe.MatchString(p.Roles[g.Group+"."+r]) {
					t.Errorf("%s: %s.%s has no color", p.ID, g.Group, r)
				}
			}
		}
		for name, role := range tokenRoles {
			if p.Colors[name] == "" || p.Colors[name] != p.Roles[role] {
				t.Errorf("%s: token %s does not resolve to %s", p.ID, name, role)
			}
		}
		if p.Appearance != "dark" && p.Appearance != "light" {
			t.Errorf("%s: appearance %q", p.ID, p.Appearance)
		}
	}
	want := []string{"sp-night", "sp-night-garoa", "sp-night-jaragua", "dracula", "everforest-dark", "gruvbox-dark", "kanagawa",
		"nord", "onedark", "rose-pine", "rose-pine-dawn", "rose-pine-moon", "tokyonight", "tokyonight-storm"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("order = %v", ids)
	}
}

// SP Night flavors carry the full upstream palette and roles.
func TestSPNightKeepsOriginalPalette(t *testing.T) {
	p, _ := lookup("sp-night")
	if len(p.Named) != 23 || p.Named["sodio"] != "#f2984a" || p.Named["estaiada"] != "#38b59e" ||
		p.Named["temporal_vivo"] != "#c3a7f6" {
		t.Fatalf("sp-night palette incomplete: %v", p.Named)
	}
	for role, want := range map[string]string{
		"ui.accent": "#f2984a", "syntax.macro": "#38b59e", "diagnostic.hint": "#5dbec4",
		"ansi.bright_magenta": "#c3a7f6", "git.untracked": "#707380",
	} {
		if p.Roles[role] != want {
			t.Errorf("%s = %s, want %s", role, p.Roles[role], want)
		}
	}
}

func writeThemes(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadUser(t *testing.T) {
	defer func() { LoadUser(t.TempDir()); Apply(Default) }()
	dir := writeThemes(t, map[string]string{
		"mine.yaml":     "label: Mine\nui:\n  accent: \"#ff0000\"\n",
		"child.yml":     "extends: mine\npalette:\n  sodio: \"#00ff00\"\n",
		"warm.yaml":     "extends: gruvbox-dark\nappearance: light\n",
		"bad.yaml":      "ui:\n  bg: \"#12345\"\n",
		"unquoted.yaml": "ui:\n  bg: #123456\n",
		"orphan.yaml":   "extends: does-not-exist\n",
		"a.yaml":        "extends: b\n",
		"b.yaml":        "extends: a\n",
		"noite.yaml":    "label: hijack\n", // legacy id stays reserved
		"sp-night.yaml": "label: hijack\n",
		"old.yaml":      "extends: garoa\n", // legacy id in extends
		"other.yaml":    "id: different\n",
		"role.yaml":     "ui:\n  acent: \"#ffffff\"\n",
		"ref.yaml":      "ui:\n  bg: missing\n",
		"key.yaml":      "colors: {}\n",
		"readme.txt":    "ignored",
	})
	errs := LoadUser(dir)
	for _, bad := range []string{"bad", "unquoted", "orphan", "a", "b", "noite", "sp-night", "other", "role", "ref", "key"} {
		found := false
		for _, err := range errs {
			if strings.Contains(err.Error(), filepath.Join(dir, bad)+".yaml") {
				found = true
			}
		}
		if !found {
			t.Errorf("no error for %s: %v", bad, errs)
		}
	}
	if len(errs) != 11 {
		t.Errorf("errors = %d: %v", len(errs), errs)
	}

	var ids []string
	for _, p := range Options() {
		if p.User {
			ids = append(ids, p.ID)
		}
	}
	if !reflect.DeepEqual(ids, []string{"child", "mine", "old", "warm"}) {
		t.Fatalf("user themes = %v", ids)
	}
	mine, _ := lookup("mine")
	spNight, _ := lookup("sp-night")
	if mine.Label != "Mine" || mine.Roles["ui.accent"] != "#ff0000" || mine.Roles["ui.bg"] != spNight.Roles["ui.bg"] {
		t.Fatalf("mine did not inherit from sp-night: %v", mine.Roles)
	}
	// overriding a palette color repaints the inherited roles that use it
	child, _ := lookup("child")
	if child.Roles["ui.cursor"] != "#00ff00" || child.Roles["ui.accent"] != "#ff0000" {
		t.Fatalf("child: cursor=%s accent=%s", child.Roles["ui.cursor"], child.Roles["ui.accent"])
	}
	if q, _ := lookup("warm"); q.Appearance != "light" || q.Roles["ui.bg"] != "#282828" {
		t.Fatalf("warm: %v %s", q.Appearance, q.Roles["ui.bg"])
	}
	// extends with a legacy SP Night id still resolves
	if old, _ := lookup("old"); old.Roles["ui.bg"] != mustLookup(t, "sp-night-garoa").Roles["ui.bg"] {
		t.Fatalf("old did not inherit from sp-night-garoa: %v", old.Roles["ui.bg"])
	}
	if spNight.Label != "SP Night" {
		t.Fatal("user file replaced a built-in theme")
	}
	if err := Apply("child"); err != nil {
		t.Fatal(err)
	}

	// a new load replaces the previous one
	LoadUser(writeThemes(t, map[string]string{"only.yaml": "label: Only\n"}))
	if _, ok := lookup("mine"); ok {
		t.Fatal("previous user theme still registered")
	}
	if errs := LoadUser(filepath.Join(dir, "does-not-exist")); len(errs) != 0 {
		t.Fatalf("missing dir: %v", errs)
	}
}

// contrast is the WCAG contrast ratio between two #rrggbb colors.
func contrast(a, b string) float64 {
	lum := func(hex string) float64 {
		var rgb [3]float64
		for i := range rgb {
			var v int
			fmt.Sscanf(hex[1+2*i:3+2*i], "%02x", &v)
			c := float64(v) / 255
			if c <= 0.03928 {
				rgb[i] = c / 12.92
			} else {
				rgb[i] = math.Pow((c+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
	}
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Every text role must be readable on the background and the selected row:
// 3:1 is the WCAG minimum for large text and components.
func TestBuiltinContrast(t *testing.T) {
	text := []string{"ui.fg", "ui.fg_dim", "ui.accent", "ui.accent_alt",
		"diagnostic.error", "diagnostic.warn", "diagnostic.info", "diagnostic.hint", "diagnostic.ok"}
	for _, p := range Options() {
		for _, surface := range []string{"ui.bg", "ui.selection"} {
			for _, role := range text {
				if c := contrast(p.Roles[role], p.Roles[surface]); c < 3 {
					t.Errorf("%s: %s on %s = %.2f:1", p.ID, role, surface, c)
				}
			}
		}
		// badges: ui.on_accent is text on a solid fill (active tab, confirm, toasts)
		for _, fill := range []string{"ui.accent", "diagnostic.ok", "diagnostic.error"} {
			if c := contrast(p.Roles["ui.on_accent"], p.Roles[fill]); c < 3 {
				t.Errorf("%s: ui.on_accent on %s = %.2f:1", p.ID, fill, c)
			}
		}
	}
}
