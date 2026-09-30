package agent

import (
	"errors"
	"fmt"
	"strings"
)

// crushProviderID is the one provider lazyagents defines in crushrc: profiles
// live in providers.json, crushrc holds only the active one.
const crushProviderID = "lazyagents"

// crushWireTypes maps a profile's wireApi to Crush's provider type; "" is the
// OpenAI-compatible API most endpoints speak.
var crushWireTypes = map[string]string{"": "openai-compat", "chat": "openai-compat", "anthropic": "anthropic"}

func (c *Crush) ProviderFile() string { return c.crushrcFile() }

// ApplyProvider writes the provider block at the end of the global crushrc:
// the provider, its model, and that model as the large one. Being last, it
// wins over the user's own `model large`, which comes back on clear.
func (c *Crush) ApplyProvider(p ProviderProfile, backupsDir string) error {
	typ, ok := crushWireTypes[p.WireAPI]
	switch {
	case p.BaseURL == "":
		return errors.New("Crush needs an endpoint (baseUrl) for a provider profile")
	case p.Model == "":
		return errors.New("Crush needs a model for a provider profile")
	case !ok:
		return fmt.Errorf("Crush does not support wireApi %q (use chat or anthropic)", p.WireAPI)
	case p.Token == "" && p.EnvKey != "" && !envNameRe.MatchString(p.EnvKey):
		return fmt.Errorf("envKey %q is not a variable name", p.EnvKey)
	}
	if err := singleLine("a provider profile", p.Name, p.BaseURL, p.Model, p.Token); err != nil {
		return err
	}
	add := []string{"provider add", crushProviderID, "--name", shQuote("lazyagents: " + p.Name),
		"--type", typ, "--base-url", shQuote(p.BaseURL)}
	switch {
	case p.Token != "":
		add = append(add, "--api-key", shQuote(p.Token))
	case p.EnvKey != "":
		add = append(add, "--api-key", `"$`+p.EnvKey+`"`) // Crush's Bash expands it at load
	}
	model := shQuote(crushProviderID + "/" + p.Model)
	body := []string{
		strings.Join(add, " "),
		"model add " + model + " --name " + shQuote(p.Model),
		"model large " + model,
	}
	return c.writeCrushBlock("provider", body, p.Token != "", backupsDir)
}

func (c *Crush) ClearProvider(backupsDir string) error {
	return c.writeCrushBlock("provider", nil, false, backupsDir)
}

// ReadProvider reads lazyagents' provider block back. The token is never
// kept: only whether one is set, or which variable holds it.
func (c *Crush) ReadProvider() (ProviderProfile, bool, error) {
	block, err := c.readCrushBlock("provider")
	if err != nil || len(block) == 0 {
		return ProviderProfile{}, false, err
	}
	var p ProviderProfile
	for _, line := range block {
		w := shWords(line)
		switch {
		case len(w) >= 3 && w[0] == "provider" && w[1] == "add":
			p.BaseURL, _ = shFlag(w, "--base-url")
			if key, ok := shFlag(w, "--api-key"); ok {
				// "$VAR" (double quotes) is a variable; a single-quoted token may
				// itself start with $
				if env, isEnv := strings.CutPrefix(key, "$"); isEnv && strings.Contains(line, `--api-key "$`) {
					p.EnvKey = env
				} else {
					p.HasToken = key != ""
				}
			}
			if typ, _ := shFlag(w, "--type"); typ == "anthropic" {
				p.WireAPI = "anthropic"
			}
		case len(w) == 3 && w[0] == "model" && w[1] == "large":
			p.Model = strings.TrimPrefix(w[2], crushProviderID+"/")
		}
	}
	return p, true, nil
}
