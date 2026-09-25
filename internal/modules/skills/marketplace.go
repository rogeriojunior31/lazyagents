package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// marketplacePath é o manifesto de marketplace do Claude Code, relativo à
// raiz da origem. Formato: https://code.claude.com/docs/en/plugin-marketplaces
const marketplacePath = ".claude-plugin/marketplace.json"

// marketplace decodifica só o que a instalação de skills usa; o resto do
// arquivo (owner, hooks, mcpServers…) é ignorado.
type marketplace struct {
	Name     string `json:"name"`
	Metadata struct {
		PluginRoot string `json:"pluginRoot"`
	} `json:"metadata"`
	Plugins []struct {
		Name   string          `json:"name"`
		Source json.RawMessage `json:"source"`
		Skills json.RawMessage `json:"skills"` // string | []string, relativos ao plugin
	} `json:"plugins"`
}

// externalSource é a forma objeto de "source": plugin fora deste repo.
type externalSource struct {
	Source  string `json:"source"` // github | url | git-subdir | npm | archive | command
	Repo    string `json:"repo"`
	URL     string `json:"url"`
	Package string `json:"package"`
}

// discoverMarketplace lê o marketplace.json de root, se existir, e devolve as
// skills dos plugins que vivem no próprio repo, com Plugin preenchido e Rel
// relativo a root (o que o update usa para reencontrar a skill). Plugins com
// origem externa não são buscados: viram notes dizendo onde instalar.
// ok=false = sem marketplace (ou sem skill nele): use a descoberta genérica.
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
			fs, _ := discoverIn(path, filepath.Base(path)) // caminho sem skill: ignora
			for _, f := range fs {
				if seen[f.Name] {
					continue // mesma skill listada por mais de um plugin
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

// pluginDir resolve a origem de um plugin para um diretório dentro de root.
// Origem externa ou que escapa de root devolve uma note em vez do dir.
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
		rel = filepath.Join(pluginRoot, rel) // nome simples resolvido por metadata.pluginRoot
	}
	dir, ok := within(root, rel)
	if !ok {
		return "", fmt.Sprintf("plugin %s skipped: source %q is outside the repository", name, rel)
	}
	return dir, ""
}

// skillPaths são os caminhos de skill de um plugin: os listados em "skills"
// que existem ou, se nenhum, o padrão <plugin>/skills.
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

// within junta rel a root e garante que o resultado não escapa de root.
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
