package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// ProviderPreviewer computes the same edits as apply/clear, without persistence.
// A nil profile previews clear. Values, credentials and foreign config stay hidden.
type ProviderPreviewer interface {
	PreviewProvider(*ProviderProfile) (ProviderPreview, error)
}

type ProviderPreview struct {
	Managed  bool                  `json:"-"` // clear-preview ownership; a clear can restore identical values
	Files    []ProviderFilePreview `json:"files"`
	Revision string                `json:"-"` // opaque review binding; never rendered or serialized
}

type ProviderFilePreview struct {
	Path    string   `json:"path"`
	Action  string   `json:"action"`
	Changes []string `json:"changes"`
}

func providerRevision(p *ProviderProfile, data ...[]byte) string {
	b, _ := json.Marshal(struct {
		Profile *ProviderProfile
		Data    [][]byte
	}{p, data})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func previewProviderFile(path string, before, after []byte, exists, remove bool, diff func([]byte, []byte) ([]string, error)) (ProviderFilePreview, error) {
	action := "update"
	switch {
	case remove && exists:
		action = "remove"
		after = nil
	case remove && !exists:
		action = "unchanged"
		after = nil
	case !exists:
		action = "create"
	case bytes.Equal(before, after):
		action = "unchanged"
	}
	changes, err := diff(before, after)
	return ProviderFilePreview{Path: path, Action: action, Changes: changes}, err
}

func jsonProviderPreview(s *settings, remove bool) (ProviderFilePreview, []byte, error) {
	after, err := s.indented()
	if err != nil {
		return ProviderFilePreview{}, nil, err
	}
	if remove {
		after = nil
	}
	f, err := previewProviderFile(s.path, s.orig, after, !s.missing, remove, recoveryJSONChanges)
	return f, after, err
}

func (c *Claude) PreviewProvider(p *ProviderProfile) (ProviderPreview, error) {
	values := map[string]string{}
	if p != nil {
		if err := ValidateProviderProfile(*p); err != nil {
			return ProviderPreview{}, err
		}
		values = map[string]string{claudeEnvBaseURL: p.BaseURL, claudeEnvModel: p.Model, claudeEnvToken: p.Token}
	}
	state, err := c.loadProviderState()
	if err != nil {
		return ProviderPreview{}, err
	}
	stateData, _ := json.Marshal(state)
	s, _, _, remove, err := c.prepareEnv(values)
	if err != nil {
		return ProviderPreview{}, err
	}
	file, after, err := jsonProviderPreview(s, remove)
	return ProviderPreview{Managed: p == nil && len(state.Written) > 0, Files: []ProviderFilePreview{file}, Revision: providerRevision(p, s.orig, after, stateData, []byte(fmt.Sprint(s.missing)))}, err
}

func codexProviderChanges(before, after []byte) ([]string, error) {
	fields := func(b []byte) map[string]string {
		top, providers := parseCodexTOML(string(b))
		out := map[string]string{}
		for _, k := range []string{"model", "model_provider"} {
			if v, ok := top[k]; ok {
				out[k] = v
			}
		}
		for _, k := range []string{"name", "base_url", "env_key", "wire_api"} {
			if v, ok := providers[codexProviderID][k]; ok {
				out["model_providers.lazyagents."+k] = v
			}
		}
		return out
	}
	return hiddenProviderChanges(fields(before), fields(after)), nil
}

func hiddenProviderChanges(before, after map[string]string) []string {
	keys := []string{}
	for k := range before {
		keys = append(keys, k)
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	changes := []string{}
	for _, k := range keys {
		a, had := before[k]
		b, has := after[k]
		switch {
		case !had:
			changes = append(changes, k+": absent -> [hidden]")
		case !has:
			changes = append(changes, k+": [hidden] -> absent")
		case a != b:
			changes = append(changes, k+": [hidden] -> [hidden] (changed)")
		}
	}
	return changes
}

func (c *Codex) PreviewProvider(p *ProviderProfile) (ProviderPreview, error) {
	var top, table []string
	setsModel := false
	if p != nil {
		if err := codexProviderFields(*p); err != nil {
			return ProviderPreview{}, err
		}
		top, table = codexProviderBlocks(*p)
		setsModel = p.Model != ""
	}
	data, err := os.ReadFile(c.ProviderFile())
	if err != nil && !os.IsNotExist(err) {
		return ProviderPreview{}, fmt.Errorf("reading %s: %w", c.ProviderFile(), err)
	}
	exists := err == nil
	out, err := editCodexProvider(data, top, table, setsModel)
	if err != nil {
		return ProviderPreview{}, err
	}
	f, err := previewProviderFile(c.ProviderFile(), data, []byte(out), exists, false, codexProviderChanges)
	return ProviderPreview{Managed: p == nil && len(f.Changes) > 0, Files: []ProviderFilePreview{f}, Revision: providerRevision(p, data, []byte(out), []byte(fmt.Sprint(exists)))}, err
}

func crushProviderChanges(before, after []byte) ([]string, error) {
	fields := func(b []byte) (map[string]string, error) {
		block, _, err := crushBlock(splitLines(string(b)), "provider")
		if err != nil {
			return nil, err
		}
		out := map[string]string{}
		for _, line := range block {
			words := shWords(line)
			switch {
			case len(words) >= 3 && words[0] == "provider" && words[1] == "add":
				for _, flag := range []string{"--base-url", "--api-key", "--type", "--name"} {
					if value, ok := shFlag(words, flag); ok {
						out["provider "+flag] = value
					}
				}
			case len(words) >= 3 && words[0] == "model":
				out["model "+words[1]] = strings.Join(words[2:], " ")
			}
		}
		return out, nil
	}
	a, err := fields(before)
	if err != nil {
		return nil, err
	}
	b, err := fields(after)
	if err != nil {
		return nil, err
	}
	return hiddenProviderChanges(a, b), nil
}

func (c *Crush) PreviewProvider(p *ProviderProfile) (ProviderPreview, error) {
	var body []string
	if p != nil {
		if err := crushProviderFields(*p); err != nil {
			return ProviderPreview{}, err
		}
		body = crushProviderBody(*p)
	}
	data, err := os.ReadFile(c.ProviderFile())
	if err != nil && !os.IsNotExist(err) {
		return ProviderPreview{}, fmt.Errorf("reading %s: %w", c.ProviderFile(), err)
	}
	exists := err == nil
	out, err := editCrushBlock(data, "provider", body)
	if err != nil {
		return ProviderPreview{}, err
	}
	f, err := previewProviderFile(c.ProviderFile(), data, []byte(out), exists, strings.TrimSpace(out) == "", crushProviderChanges)
	return ProviderPreview{Managed: p == nil && len(f.Changes) > 0, Files: []ProviderFilePreview{f}, Revision: providerRevision(p, data, []byte(out), []byte(fmt.Sprint(exists)))}, err
}

func (p *Pi) PreviewProvider(pr *ProviderProfile) (ProviderPreview, error) {
	state, err := p.loadProviderState()
	if err != nil {
		return ProviderPreview{}, err
	}
	stateData, _ := json.Marshal(state)
	var models, settings *settings
	remove := false
	if pr != nil {
		if err := piProviderFields(*pr); err != nil {
			return ProviderPreview{}, err
		}
		models, settings, _, err = p.prepareApply(piProfileConfig(*pr), pr.Model)
	} else {
		models, settings, remove, err = p.prepareClear()
	}
	if err != nil {
		return ProviderPreview{}, err
	}
	mf, ma, err := jsonProviderPreview(models, remove)
	if err != nil {
		return ProviderPreview{}, err
	}
	sf, sa, err := jsonProviderPreview(settings, false)
	if err != nil {
		return ProviderPreview{}, err
	}
	if pr == nil {
		if !remove && !piProviderChanged(models, "providers") {
			mf.Action = "unchanged"
			mf.Changes = nil
			ma = models.orig
		}
		if !piProviderChanged(settings, "defaultProvider") && !piProviderChanged(settings, "defaultModel") {
			sf.Action = "unchanged"
			sf.Changes = nil
			sa = settings.orig
		}
	}
	return ProviderPreview{Managed: pr == nil && (len(mf.Changes) > 0 || len(sf.Changes) > 0), Files: []ProviderFilePreview{mf, sf}, Revision: providerRevision(pr, models.orig, ma, settings.orig, sa, stateData, []byte(fmt.Sprint(models.missing, settings.missing)))}, nil
}
