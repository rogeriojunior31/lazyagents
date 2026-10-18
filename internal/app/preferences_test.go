package app

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui"
)

func TestPreferencesMergeForeignConfigAndPersistOnlyOwnedFields(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	before := "# header\ntheme: sp-night # theme choice\nlibraryDir: {future: preserve} # unrelated shape\ntui: # layout\n  splash: false # no splash\n  splashSeconds: 0\n  future: keep # other owner\n  tabs: [agents, skills, missing-plugin] # order\n  hidden: [missing-plugin]\nskills:\n  installTargets: [codex]\nplugin:\n  token: private-foreign-sentinel\n"
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigPath(), []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{Deps: &feature.Deps{Paths: paths}}
	catalog := []tui.PreferenceTab{{ID: "skills", Title: "Skills"}, {ID: "agents", Title: "Agents"}}
	p, err := a.loadPreferences(catalog)
	if err != nil || p.StartTab != "agents" || len(p.Tabs) != 3 || !p.Tabs[2].Unavailable || !p.Tabs[2].Hidden {
		t.Fatalf("preferences = %+v %v", p, err)
	}
	data, _ := json.Marshal(p)
	if strings.Contains(string(data), "private-foreign-sentinel") {
		t.Fatal("preferences included foreign config values")
	}
	if err := os.WriteFile(paths.ConfigPath(), []byte(strings.Replace(before, "future: keep", "future: changed elsewhere", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	p.Theme = "sp-night-garoa"
	p.StartTab = "skills"
	p.Tabs[0], p.Tabs[1] = p.Tabs[1], p.Tabs[0]
	p.Tabs[1].Hidden = true
	p, err = a.savePreferences(p, catalog)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(paths.ConfigPath())
	for _, want := range []string{"# header", "# theme choice", "libraryDir: {future: preserve} # unrelated shape", "splash: false # no splash", "splashSeconds: 0", "future: changed elsewhere # other owner", "installTargets: [codex]", "token: private-foreign-sentinel", "# order"} {
		if !strings.Contains(string(after), want) {
			t.Fatalf("save lost %q: %s", want, after)
		}
	}
	cfg, layout, err := readPreferences(paths.ConfigPath())
	if err != nil || cfg.Theme != p.Theme || layout.StartTab != "skills" || !reflect.DeepEqual(layout.Tabs, []string{"skills", "agents", "missing-plugin"}) || !slices.Contains(layout.Hidden, "agents") {
		t.Fatalf("saved fields = %+v %v", layout, err)
	}
	if p.Revision != preferenceRevision(cfg, layout) {
		t.Fatal("save returned a stale review revision")
	}
	if _, err = a.savePreferences(p, catalog); err != nil {
		t.Fatalf("repeat save failed: %v", err)
	}
	loaded, err := LoadWith(paths, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	mods, opts := loaded.Layout()
	if mods[opts.Start].ID() != "skills" || mods[0].ID() != "skills" {
		t.Fatal("saved layout was not honored on restart")
	}
	for _, mod := range mods {
		if mod.ID() == "agents" {
			t.Fatal("hidden tab reappeared after restart")
		}
	}
}

func TestPreferencesRefuseStaleOwnedFieldsAndUnsafeSelections(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	a := &App{Deps: &feature.Deps{Paths: paths}}
	catalog := []tui.PreferenceTab{{ID: "skills", Title: "Skills"}, {ID: "agents", Title: "Agents"}}
	p, err := a.loadPreferences(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*tui.Preferences){
		func(p *tui.Preferences) { p.Theme = "invalid-private-sentinel" },
		func(p *tui.Preferences) { p.Tabs[0].Hidden = true; p.Tabs[1].Hidden = true },
		func(p *tui.Preferences) { p.StartTab = "unknown" },
		func(p *tui.Preferences) { p.Tabs[1].ID = p.Tabs[0].ID },
	} {
		draft := p
		draft.Tabs = slices.Clone(p.Tabs)
		edit(&draft)
		if _, err := a.savePreferences(draft, catalog); err == nil {
			t.Fatal("unsafe preference draft accepted")
		}
		if _, err := os.Stat(paths.ConfigPath()); !os.IsNotExist(err) {
			t.Fatal("invalid preferences wrote config")
		}
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	changed := []byte("theme: sp-night-garoa\n")
	if err := os.WriteFile(paths.ConfigPath(), changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.savePreferences(p, catalog); err == nil || !strings.Contains(err.Error(), "changed on disk") {
		t.Fatalf("stale draft accepted: %v", err)
	}
	after, _ := os.ReadFile(paths.ConfigPath())
	if string(after) != string(changed) {
		t.Fatal("stale save replaced current preferences")
	}
	invalid := []byte("tui: [private-foreign-sentinel\n")
	if err := os.WriteFile(paths.ConfigPath(), invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.loadPreferences(catalog); err == nil || strings.Contains(err.Error(), "private-foreign-sentinel") {
		t.Fatalf("unsafe parser error: %v", err)
	}
}
