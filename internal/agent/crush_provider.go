package agent

import (
	"errors"
	"fmt"
	"os"
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
	if err := crushProviderFields(p); err != nil {
		return err
	}
	return c.writeCrushBlock("provider", crushProviderBody(p), p.Token != "", backupsDir)
}

func (c *Crush) ClearProvider(backupsDir string) error {
	return c.writeCrushBlock("provider", nil, false, backupsDir)
}

// ReadProvider reads lazyagents' provider block back. The token is never
// kept: only whether one is set, or which variable holds it.
func (c *Crush) ReadProvider() (ProviderProfile, bool, error) {
	block, err := c.readCrushBlock("provider")
	if err != nil {
		return ProviderProfile{}, false, err
	}
	if len(block) == 0 {
		return c.readExternalJSONProvider()
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
				if env := providerEnvReference(key); env != "" && strings.Contains(line, `--api-key "$`) {
					p.EnvKey = env
				} else {
					p.HasToken = key != ""
				}
			}
			if typ, _ := shFlag(w, "--type"); typ != "" && typ != "openai-compat" {
				p.WireAPI = typ
			}
		case len(w) == 3 && w[0] == "model" && w[1] == "large":
			p.Model = strings.TrimPrefix(w[2], crushProviderID+"/")
		}
	}
	return p, true, nil
}

func crushProviderFields(p ProviderProfile) error {
	if err := ValidateProviderProfile(p); err != nil {
		return err
	}
	_, ok := crushWireTypes[p.WireAPI]
	switch {
	case p.BaseURL == "":
		return errors.New("Crush needs an endpoint (baseUrl) for a provider profile")
	case p.Model == "":
		return errors.New("Crush needs a model for a provider profile")
	case !ok:
		return fmt.Errorf("Crush does not support wireApi %q (use chat or anthropic)", p.WireAPI)
	}
	if err := singleLine("a provider profile", p.Name, p.BaseURL, p.Model, p.Token); err != nil {
		return err
	}
	return nil
}

func (c *Crush) ValidateProvider(p ProviderProfile) (ProviderValidation, error) {
	credential := "No profile credential is written; the endpoint may require authentication."
	if p.Token != "" || p.HasToken {
		credential = "Crush writes the saved token to crushrc; it takes precedence over envKey."
	} else if p.EnvKey != "" {
		credential = fmt.Sprintf("Crush expands %s when loading crushrc; export it in the Crush process's environment.", p.EnvKey)
	}
	v := providerValidation(ProviderFiles(c), credential)
	if err := crushProviderFields(p); err != nil {
		return v, err
	}
	v.Notes = append([]string{fmt.Sprintf("Crush uses %s and selects the profile model as its large model.", crushWireTypes[p.WireAPI])}, v.Notes...)
	_, err := c.readCrushBlock("provider")
	return v, err
}

func crushProviderBody(p ProviderProfile) []string {
	typ := crushWireTypes[p.WireAPI]
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
	return body
}

// Executable crushrc configuration cannot be evaluated safely during import.
// Only an explicit JSON connection with no executable rc overrides is adopted.
func (c *Crush) readExternalJSONProvider() (ProviderProfile, bool, error) {
	data, err := os.ReadFile(c.ProviderFile())
	if err != nil && !os.IsNotExist(err) {
		return ProviderProfile{}, false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return ProviderProfile{}, false, nil
		}
	}
	var cfg struct {
		Models map[string]struct {
			Model    string `json:"model"`
			Provider string `json:"provider"`
		} `json:"models"`
		Providers map[string]struct {
			BaseURL string `json:"base_url"`
			Type    string `json:"type"`
			APIKey  piKey  `json:"api_key"`
			Disable bool   `json:"disable"`
		} `json:"providers"`
	}
	if err := decodeJSONFile(c.configFile(), &cfg); err != nil {
		if os.IsNotExist(err) {
			return ProviderProfile{}, false, nil
		}
		return ProviderProfile{}, false, fmt.Errorf("reading %s: %w", c.configFile(), err)
	}
	large := cfg.Models["large"]
	provider, ok := cfg.Providers[large.Provider]
	if !ok || provider.Disable || provider.BaseURL == "" || large.Model == "" {
		return ProviderProfile{}, false, nil
	}
	wire := ""
	switch provider.Type {
	case "openai-compat":
		wire = "chat"
	case "anthropic":
		wire = "anthropic"
	default:
		return ProviderProfile{}, false, nil
	}
	return ProviderProfile{Name: large.Provider, BaseURL: provider.BaseURL, Model: large.Model, WireAPI: wire, EnvKey: provider.APIKey.env, HasToken: provider.APIKey.has && provider.APIKey.env == ""}, true, nil
}
