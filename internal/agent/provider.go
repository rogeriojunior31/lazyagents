package agent

import (
	"fmt"
	"net/url"
)

// ProviderProfile is an endpoint/model profile applied to an agent's live
// config (cc-switch style). Token is the only secret: display only Redacted().
type ProviderProfile struct {
	Name    string `json:"name"`              // profile name in lazyagents
	BaseURL string `json:"baseUrl,omitempty"` // compatible endpoint
	Model   string `json:"model,omitempty"`   // default model ("" = leave as is)
	Token   string `json:"token,omitempty"`   // SECRET: only in providers.json (0600) and the agent config
	EnvKey  string `json:"envKey,omitempty"`  // env var holding the token, for agents that refuse tokens in files
	WireAPI string `json:"wireApi,omitempty"` // Codex: responses; Pi/Crush: protocol; Claude ignores it

	// HasToken, set by Redacted and by reading the live config, says a token
	// exists without revealing it.
	HasToken bool `json:"hasToken,omitempty"`
}

// Redacted returns the profile without the secret, with HasToken instead. Use
// it in the TUI, --json and logs. Idempotent: keeps a HasToken read from config.
func (p ProviderProfile) Redacted() ProviderProfile {
	p.HasToken = p.HasToken || p.Token != ""
	p.Token = ""
	return p
}

// ProviderHost is implemented by adapters that can apply a provider profile to
// their config. Optional, by type assertion.
type ProviderHost interface {
	// ProviderFile is the config file Apply/Clear write.
	ProviderFile() string
	// ReadProvider returns the applied provider. Token is NEVER set, only
	// HasToken. ok=false when nothing is applied.
	ReadProvider() (p ProviderProfile, ok bool, err error)
	// ApplyProvider writes the profile, backing the live file up into backupsDir.
	ApplyProvider(p ProviderProfile, backupsDir string) error
	// ClearProvider undoes ApplyProvider, keeping what lazyagents does not manage.
	ClearProvider(backupsDir string) error
}

// ProviderValidator prepares the destinations and requirements without writing.
// ApplyProvider must still read and validate live files again at write time.
type ProviderValidator interface {
	ValidateProvider(ProviderProfile) (ProviderValidation, error)
}

// ProviderValidation contains no credential values or native configuration.
type ProviderValidation struct {
	Files []string `json:"files"`
	Notes []string `json:"notes"`
}

// ValidateProviderProfile checks shared fields, including hand-edited libraries.
func ValidateProviderProfile(p ProviderProfile) error {
	if p.BaseURL != "" {
		u, err := url.Parse(p.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("endpoint: use an http:// or https:// URL without embedded credentials or a fragment")
		}
	}
	if p.EnvKey != "" && !envNameRe.MatchString(p.EnvKey) {
		return fmt.Errorf("envKey must be an environment variable name (letters, digits and underscores, starting with a letter or underscore)")
	}
	return nil
}

func providerValidation(files []string, credential string) ProviderValidation {
	return ProviderValidation{Files: files, Notes: []string{
		credential,
		"Credentials and endpoint compatibility have not been authenticated or tested over the network.",
		"Start a new agent session to load these settings; running sessions may keep their previous configuration.",
	}}
}
