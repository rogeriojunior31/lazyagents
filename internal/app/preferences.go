package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Only these fields are owned by the preferences editor; splash and foreign
// tui fields survive a save, including values added while the editor is open.
type preferenceConfig struct {
	StartTab string   `yaml:"startTab"`
	Tabs     []string `yaml:"tabs"`
	Hidden   []string `yaml:"hidden"`
}

func preferenceRevision(c core.Config, layout preferenceConfig) string {
	data, _ := json.Marshal(struct {
		Theme  string
		Layout preferenceConfig
	}{c.Theme, layout})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func readPreferences(path string) (core.Config, preferenceConfig, error) {
	cfg, err := core.ReadConfig(path)
	var layout preferenceConfig
	if err != nil || cfg.Section("tui", &layout) != nil {
		return core.Config{}, layout, errors.New("Cannot read preferences. Review config.yaml syntax and file access locally, then reopen settings.")
	}
	return cfg, layout, nil
}

func (a *App) loadPreferences(catalog []tui.PreferenceTab) (tui.Preferences, error) {
	cfg, layout, err := readPreferences(a.Deps.Paths.ConfigPath())
	if err != nil {
		return tui.Preferences{}, err
	}
	p := tui.Preferences{Theme: cfg.Theme, StartTab: layout.StartTab, Revision: preferenceRevision(cfg, layout)}
	if p.Theme == "" {
		p.Theme = theme.Default
	}
	for _, option := range theme.Options() {
		p.Themes = append(p.Themes, option.ID)
	}
	if !slices.Contains(p.Themes, p.Theme) {
		p.Themes = append(p.Themes, p.Theme)
	}
	ids := slices.Clone(layout.Tabs)
	for _, tab := range catalog {
		if !slices.Contains(ids, tab.ID) {
			ids = append(ids, tab.ID)
		}
	}
	for _, id := range layout.Hidden {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		tab := tui.PreferenceTab{ID: id, Title: id + " (not loaded)", Hidden: slices.Contains(layout.Hidden, id), Unavailable: true}
		for _, known := range catalog {
			if known.ID == id {
				tab.Title, tab.Unavailable = known.Title, false
				break
			}
		}
		p.Tabs = append(p.Tabs, tab)
	}
	if !slices.ContainsFunc(p.Tabs, func(t tui.PreferenceTab) bool { return t.ID == p.StartTab && !t.Hidden && !t.Unavailable }) {
		p.StartTab = ""
		for _, tab := range p.Tabs {
			if !tab.Hidden && !tab.Unavailable {
				p.StartTab = tab.ID
				break
			}
		}
	}
	return p, nil
}

func (a *App) savePreferences(p tui.Preferences, catalog []tui.PreferenceTab) (tui.Preferences, error) {
	path := a.Deps.Paths.ConfigPath()
	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return p, errors.New("Cannot read config.yaml; preferences were not saved.")
	}
	cfg, layout, err := readPreferences(path)
	if err != nil {
		return p, err
	}
	if p.Revision != preferenceRevision(cfg, layout) {
		return p, errors.New("Preferences changed on disk. Reopen settings to review them before saving.")
	}
	current, err := a.loadPreferences(catalog)
	if err != nil {
		return p, err
	}
	if !slices.Contains(current.Themes, p.Theme) {
		return p, errors.New("Choose an available theme before saving.")
	}
	if len(p.Tabs) != len(current.Tabs) {
		return p, errors.New("The tab list changed. Reopen settings before saving.")
	}
	seen := map[string]bool{}
	var order, hidden []string
	startVisible := false
	for _, tab := range p.Tabs {
		if seen[tab.ID] || !slices.ContainsFunc(current.Tabs, func(t tui.PreferenceTab) bool { return t.ID == tab.ID }) {
			return p, errors.New("The tab list changed. Reopen settings before saving.")
		}
		seen[tab.ID] = true
		order = append(order, tab.ID)
		if tab.Hidden {
			hidden = append(hidden, tab.ID)
		} else if tab.ID == p.StartTab {
			startVisible = slices.ContainsFunc(catalog, func(t tui.PreferenceTab) bool { return t.ID == tab.ID })
		}
	}
	if !startVisible {
		return p, errors.New("Choose a visible initial tab; at least one tab must stay visible.")
	}
	if hidden == nil {
		hidden = []string{}
	}
	cfg.Theme = p.Theme
	for _, field := range []struct {
		key   string
		value any
	}{{"startTab", p.StartTab}, {"tabs", order}, {"hidden", hidden}} {
		if err := cfg.SetSectionValue("tui", field.key, field.value); err != nil {
			return p, errors.New("Cannot update the tui configuration; preferences were not saved.")
		}
	}
	if err := cfg.Save(path, before); err != nil {
		return p, errors.New("Cannot save preferences: config.yaml changed or could not be written. Reopen settings and check file access.")
	}
	p.Revision = preferenceRevision(cfg, preferenceConfig{StartTab: p.StartTab, Tabs: order, Hidden: hidden})
	return p, nil
}
