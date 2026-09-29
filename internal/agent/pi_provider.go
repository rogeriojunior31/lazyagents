package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// piProviderID is the one custom provider lazyagents manages in pi's
// models.json: profiles live in providers.json, models.json holds the active one.
const piProviderID = "lazyagents"

// piNoKey is the apiKey of a profile without a token: pi hides a custom
// provider's models until it can resolve a credential, so keyless endpoints
// (Ollama) get a dummy one, as pi's docs do.
const piNoKey = "lazyagents-no-key"

// piWireAPIs maps a profile's wireApi to pi's api; "" is the OpenAI-compatible
// chat completions most endpoints speak.
var piWireAPIs = map[string]string{
	"":                   "openai-completions",
	"chat":               "openai-completions",
	"openai-completions": "openai-completions",
	"responses":          "openai-responses",
	"openai-responses":   "openai-responses",
	"anthropic":          "anthropic-messages",
	"anthropic-messages": "anthropic-messages",
}

// piProviderState is what the user had as default provider and model before a
// profile replaced them, kept in lazyagents' data dir so pi's files carry no
// marker. No secret goes here.
type piProviderState struct {
	Applied      bool    `json:"applied"`
	PrevProvider *string `json:"prevProvider,omitempty"` // nil: the key was absent
	PrevModel    *string `json:"prevModel,omitempty"`
	CreatedFile  bool    `json:"createdFile,omitempty"` // models.json did not exist before
}

// piProviderConfig is the models.json entry lazyagents writes.
type piProviderConfig struct {
	Name    string          `json:"name"`
	BaseURL string          `json:"baseUrl"`
	API     string          `json:"api"`
	APIKey  string          `json:"apiKey,omitempty"`
	Models  []piModelConfig `json:"models"`
}

type piModelConfig struct {
	ID string `json:"id"`
}

// piKey records how an apiKey is given without keeping it: a "$VAR" or
// "${VAR}" reference keeps the variable name, a literal only that it exists.
type piKey struct {
	has bool
	env string
}

func (k *piKey) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil || s == "" || s == piNoKey {
		return nil
	}
	k.has = true
	if strings.HasPrefix(s, "$") {
		k.env = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(s, "$"), "{"), "}")
	}
	return nil
}

func (p *Pi) ProviderFile() string { return filepath.Join(p.agentDir(), "models.json") }

func (p *Pi) settingsFile() string { return filepath.Join(p.agentDir(), "settings.json") }

func (p *Pi) ReadProvider() (ProviderProfile, bool, error) {
	s, err := readSettings(p.settingsFile())
	if err != nil {
		return ProviderProfile{}, false, err
	}
	var def string
	if ok, _ := s.get("defaultProvider", &def); !ok || def != piProviderID {
		return ProviderProfile{}, false, nil // not applied, or the user switched away in pi
	}
	var models struct {
		Providers map[string]struct {
			BaseURL string          `json:"baseUrl"`
			API     string          `json:"api"`
			APIKey  piKey           `json:"apiKey"`
			Models  []piModelConfig `json:"models"`
		} `json:"providers"`
	}
	if err := decodeJSONFile(p.ProviderFile(), &models); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ProviderProfile{}, false, fmt.Errorf("reading %s: %w", p.ProviderFile(), err)
	}
	cfg, ok := models.Providers[piProviderID]
	if !ok {
		return ProviderProfile{}, false, nil
	}
	pr := ProviderProfile{BaseURL: cfg.BaseURL, HasToken: cfg.APIKey.has && cfg.APIKey.env == "", EnvKey: cfg.APIKey.env}
	var model string
	if ok, _ := s.get("defaultModel", &model); ok {
		pr.Model = model
	} else if len(cfg.Models) > 0 {
		pr.Model = cfg.Models[0].ID
	}
	switch cfg.API {
	case "openai-responses":
		pr.WireAPI = "responses"
	case "anthropic-messages":
		pr.WireAPI = "anthropic"
	}
	return pr, true, nil
}

// ApplyProvider writes the profile as pi's "lazyagents" provider in models.json
// and makes it the default in settings.json. Pi only offers a custom
// provider's declared models, so the profile needs an endpoint and a model.
func (p *Pi) ApplyProvider(pr ProviderProfile, backupsDir string) error {
	api, ok := piWireAPIs[pr.WireAPI]
	switch {
	case pr.BaseURL == "":
		return errors.New("Pi needs an endpoint (baseUrl) for a provider profile")
	case pr.Model == "":
		return errors.New("Pi needs a model for a provider profile: it only offers the models a custom provider declares")
	case !ok:
		return fmt.Errorf("Pi does not support wireApi %q (use chat, responses or anthropic)", pr.WireAPI)
	}
	cfg := piProviderConfig{Name: "lazyagents: " + pr.Name, BaseURL: pr.BaseURL, API: api,
		Models: []piModelConfig{{ID: pr.Model}}}
	switch {
	case pr.Token != "":
		cfg.APIKey = pr.Token
	case pr.EnvKey != "":
		cfg.APIKey = "$" + pr.EnvKey // pi resolves $VAR itself
	default:
		cfg.APIKey = piNoKey
	}

	st, err := p.loadProviderState()
	if err != nil {
		return err
	}
	models, err := readSettings(p.ProviderFile())
	if err != nil {
		return err
	}
	settings, err := readSettings(p.settingsFile())
	if err != nil {
		return err
	}
	// Record the user's defaults unless ours are the current ones: after the
	// user switched away in pi, their new choice is what clear gives back.
	if cur := settingString(settings, "defaultProvider"); !st.Applied || cur == nil || *cur != piProviderID {
		created := models.missing || (st.Applied && st.CreatedFile)
		st = piProviderState{Applied: true, CreatedFile: created}
		if cur == nil || *cur != piProviderID { // a lost state file must not make "lazyagents" the previous one
			st.PrevProvider = cur
			st.PrevModel = settingString(settings, "defaultModel")
		}
	}
	providers, err := providersObject(models)
	if err != nil {
		return err
	}
	if err := providers.set(piProviderID, cfg); err != nil {
		return err
	}
	if err := models.set("providers", providers); err != nil {
		return err
	}
	models.perm = 0o600 // it may carry the token now
	if err := settings.set("defaultProvider", piProviderID); err != nil {
		return err
	}
	if err := settings.set("defaultModel", pr.Model); err != nil {
		return err
	}
	// Record the user's defaults before touching pi's files: a failed save
	// below still leaves them known.
	if err := p.saveProviderState(st); err != nil {
		return err
	}
	if err := models.save(backupsDir); err != nil {
		return err
	}
	return settings.save(backupsDir)
}

// ClearProvider removes the "lazyagents" provider and gives the default back
// to what it was, unless the user already switched away in pi.
func (p *Pi) ClearProvider(backupsDir string) error {
	st, err := p.loadProviderState()
	if err != nil {
		return err
	}
	models, err := readSettings(p.ProviderFile())
	if err != nil {
		return err
	}
	if providers, err := providersObject(models); err == nil {
		if _, ours := providers.raw(piProviderID); ours {
			providers.delete(piProviderID)
			var set any = providers
			if providers.empty() {
				set = nil
			}
			if err := models.set("providers", set); err != nil {
				return err
			}
			if models.object.empty() && st.CreatedFile {
				if backupsDir != "" {
					if _, err := fsutil.Backup(models.path, backupsDir); err != nil {
						return err
					}
				}
				if err := os.Remove(models.path); err != nil {
					return fmt.Errorf("removing %s: %w", models.path, err)
				}
			} else if err := models.save(backupsDir); err != nil {
				return err
			}
		}
	}
	settings, err := readSettings(p.settingsFile())
	if err != nil {
		return err
	}
	if def := settingString(settings, "defaultProvider"); def != nil && *def == piProviderID {
		if err := restoreSetting(settings, "defaultProvider", st.PrevProvider); err != nil {
			return err
		}
		if err := restoreSetting(settings, "defaultModel", st.PrevModel); err != nil {
			return err
		}
		if err := settings.save(backupsDir); err != nil {
			return err
		}
	}
	return p.saveProviderState(piProviderState{})
}

// providersObject is models.json "providers", keeping every other provider as
// it is on disk.
func providersObject(s *settings) (*object, error) {
	raw, ok := s.raw("providers")
	if !ok || strings.TrimSpace(string(raw)) == "null" {
		return &object{}, nil
	}
	o, err := decodeObject(raw)
	if err != nil {
		return nil, fmt.Errorf("reading %s: providers: %w", s.path, err)
	}
	return o, nil
}

func settingString(s *settings, key string) *string {
	var v string
	if ok, err := s.get(key, &v); ok && err == nil {
		return &v
	}
	return nil
}

func restoreSetting(s *settings, key string, prev *string) error {
	if prev == nil {
		return s.set(key, nil)
	}
	return s.set(key, *prev)
}

func (p *Pi) loadProviderState() (piProviderState, error) {
	var st piProviderState
	if p.ProviderState == "" {
		if p.providerMem != nil {
			st = *p.providerMem
		}
		return st, nil
	}
	data, err := os.ReadFile(p.ProviderState)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("reading %s: %w", p.ProviderState, err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("reading %s: %w", p.ProviderState, err)
	}
	return st, nil
}

func (p *Pi) saveProviderState(st piProviderState) error {
	if p.ProviderState == "" {
		p.providerMem = &st
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("writing %s: %w", p.ProviderState, err)
	}
	return fsutil.WriteAtomic(p.ProviderState, append(data, '\n'), 0o600)
}
