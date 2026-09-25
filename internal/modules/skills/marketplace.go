package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// marketplacePath is the Claude Code marketplace manifest, relative to the
// source root. Format: https://code.claude.com/docs/en/plugin-marketplaces
const marketplacePath = ".claude-plugin/marketplace.json"

// marketplace decodes only what skill install uses; the rest (owner, hooks,
// mcpServers…) is ignored.
type marketplace struct {
	Name     string `json:"name"`
	Metadata struct {
		PluginRoot string `json:"pluginRoot"`
	} `json:"metadata"`
	Plugins []struct {
		Name   string          `json:"name"`
		Source json.RawMessage `json:"source"`
		Skills json.RawMessage `json:"skills"` // string | []string, relative to the plugin
	} `json:"plugins"`
}

// externalSource is the object form of "source": a plugin outside this repo.
type externalSource struct {
	Source  string `json:"source"` // github | url | git-subdir | npm | archive | command
	Repo    string `json:"repo"`
	URL     string `json:"url"`
	Package string `json:"package"`
}

// discoverMarketplace reads root's marketplace.json and returns the skills of
// in-repo plugins, with Plugin set and Rel relative to root (update uses it to
// find the skill again). External plugins are not fetched: they become notes.
// ok=false means no marketplace (or no skill in it): use generic discovery.
func discoverMarketplace(root string) (found []Found, notes []string, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(root, marketplacePath))
	if err != nil {
		return nil, nil, false, nil
	}
	var mp marketplace
	if err := json.Unmarshal(data, &mp); err != nil {
		return nil, nil, false, fmt.Errorf("invalid %s: %w", marketplacePath, err)
	}
	seen := map[string]bool{}
	for _, pl := range mp.Plugins {
		dir, note := pluginDir(root, mp.Metadata.PluginRoot, pl.Name, pl.Source)
		if note != "" {
			notes = append(notes, note)
			continue
		}
		for _, path := range skillPaths(root, dir, pl.Skills) {
			fs, _ := discoverIn(path, filepath.Base(path)) // path without a skill: skip
			for _, f := range fs {
				if seen[f.Name] {
					continue // same skill listed by more than one plugin
				}
				seen[f.Name] = true
				f.Rel, _ = filepath.Rel(root, f.SrcDir)
				f.Plugin = pl.Name
				found = append(found, f)
			}
		}
	}
	return found, notes, len(found) > 0, nil
}

// pluginDir resolves a plugin source to a directory inside root; an external
// source or one escaping root returns a note instead.
func pluginDir(root, pluginRoot, name string, raw json.RawMessage) (string, string) {
	var rel string
	if err := json.Unmarshal(raw, &rel); err != nil {
		var ext externalSource
		_ = json.Unmarshal(raw, &ext)
		switch where := firstNonEmpty(ext.Repo, ext.URL, ext.Package); {
		case ext.Source == "github" || ext.Source == "url" || ext.Source == "git-subdir":
			return "", fmt.Sprintf("plugin %s comes from another repository: install %s", name, where)
		default:
			return "", fmt.Sprintf("plugin %s skipped: unsupported source %q", name, ext.Source)
		}
	}
	if !strings.HasPrefix(rel, "./") && rel != "." && pluginRoot != "" {
		rel = filepath.Join(pluginRoot, rel) // bare name resolved via metadata.pluginRoot
	}
	dir, ok := within(root, rel)
	if !ok {
		return "", fmt.Sprintf("plugin %s skipped: source %q is outside the repository", name, rel)
	}
	return dir, ""
}

// skillPaths are a plugin's skill paths: those listed in "skills" that exist,
// or else the default <plugin>/skills.
func skillPaths(root, dir string, raw json.RawMessage) []string {
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		var one string
		if json.Unmarshal(raw, &one) == nil && one != "" {
			list = []string{one}
		}
	}
	var out []string
	for _, rel := range list {
		if p, ok := within(root, filepath.Join(dir, rel)); ok {
			if _, err := os.Stat(p); err == nil {
				out = append(out, p)
			}
		}
	}
	if len(out) == 0 {
		out = []string{filepath.Join(dir, "skills")}
	}
	return out
}

// within joins rel to root and ensures the result stays inside root.
func within(root, rel string) (string, bool) {
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, rel)
	}
	r, err := filepath.Rel(root, p)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	return p, true
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
