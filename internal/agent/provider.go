package agent

// ProviderProfile is an endpoint/model profile applied to an agent's live
// config (cc-switch style). Token is the only secret: display only Redacted().
type ProviderProfile struct {
	Name    string `json:"name"`              // profile name in lazyagents
	BaseURL string `json:"baseUrl,omitempty"` // compatible endpoint
	Model   string `json:"model,omitempty"`   // default model ("" = leave as is)
	Token   string `json:"token,omitempty"`   // SECRET: only in providers.json (0600) and the agent config
	EnvKey  string `json:"envKey,omitempty"`  // env var holding the token, for agents that refuse tokens in files
	WireAPI string `json:"wireApi,omitempty"` // Codex: responses ("" = Codex default)

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
