package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderEnvironmentReader inspects the configured snapshot and this process,
// never shell profiles, credential commands or a running agent's environment.
type ProviderEnvironmentReader interface {
	ProviderEnvironment() ([]ProviderDiagnostic, error)
}

// ProviderDiagnostic contains names and origins only. Values must stay inside
// the adapter, before any report, JSON serialization or rendering takes place.
type ProviderDiagnostic struct {
	Code        string `json:"code"`
	Variable    string `json:"variable,omitempty"`
	Source      string `json:"source"`
	OtherSource string `json:"other_source,omitempty"`
	Explanation string `json:"explanation"`
	NextStep    string `json:"next_step"`
}

func missingProviderEnvironment(key, source, note string) []ProviderDiagnostic {
	if key == "" {
		return nil
	}
	if !envNameRe.MatchString(key) {
		return []ProviderDiagnostic{{Code: "invalid_credential_reference", Source: source,
			Explanation: "The configured credential reference is not a supported environment variable name. Its value is hidden.",
			NextStep:    "Review the native credential reference locally; use a variable name containing letters, digits and underscores, starting with a letter or underscore."}}
	}
	if value, present := os.LookupEnv(key); present && strings.TrimSpace(value) != "" {
		return nil
	}
	return []ProviderDiagnostic{{Code: "missing_credential_environment", Variable: key,
		Source: source, OtherSource: "current process environment",
		Explanation: "The credential variable is unset or empty in the lazyagents process. " + note,
		NextStep:    fmt.Sprintf("Supply %s in the environment used to launch the agent, then restart lazyagents from that environment to check again. Review the native authentication mode; presence does not prove authentication.", key)}}
}

func (c *Claude) ProviderEnvironment() ([]ProviderDiagnostic, error) {
	if os.Getenv("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST") != "" {
		return nil, nil // Hosted sessions have their own routing policy.
	}
	s, err := readSettings(c.ProviderFile())
	if err != nil {
		return nil, err
	}
	// Decode only public routing fields; credential literals are not inspected.
	var env struct {
		Endpoint *string `json:"ANTHROPIC_BASE_URL"`
		Model    *string `json:"ANTHROPIC_MODEL"`
	}
	if _, err := s.get("env", &env); err != nil {
		return nil, err
	}
	var out []ProviderDiagnostic
	for _, field := range []struct {
		key   string
		value *string
	}{{claudeEnvBaseURL, env.Endpoint}, {claudeEnvModel, env.Model}} {
		inherited, present := os.LookupEnv(field.key)
		if field.value == nil || !present || inherited == *field.value {
			continue
		}
		out = append(out, ProviderDiagnostic{Code: "environment_config_conflict", Variable: field.key,
			Source: c.ProviderFile() + " (env)", OtherSource: "current process environment",
			Explanation: "The current process and settings.env define different values. Claude normally replaces the inherited value with settings.env; session and managed policy can change which source applies.",
			NextStep:    "Review both sources and Claude's session policy before editing either one. Relaunch Claude after an intentional change; lazyagents does not unset variables or inspect shell startup files."})
	}
	var model string
	if _, err := s.get("model", &model); err != nil {
		return nil, err
	}
	value, source := os.Getenv(claudeEnvModel), "current process environment"
	if env.Model != nil {
		value, source = *env.Model, c.ProviderFile()+" (env)"
	}
	if model != "" && value != "" && model != value {
		out = append(out, ProviderDiagnostic{Code: "environment_model_override", Variable: claudeEnvModel,
			Source: source, OtherSource: c.ProviderFile() + " (model)",
			Explanation: "ANTHROPIC_MODEL differs from the model setting and has priority when applied. CLI flags, /model, managed settings and session policy can override this snapshot.",
			NextStep:    "Review ANTHROPIC_MODEL and the model setting together, then start a new Claude session with the intended model."})
	}
	return out, nil
}

func (c *Codex) ProviderEnvironment() ([]ProviderDiagnostic, error) {
	p, active, err := c.ReadProvider()
	if err != nil || !active {
		return nil, err
	}
	return missingProviderEnvironment(p.EnvKey, c.ProviderFile()+" (selected provider env_key)",
		"Codex reads the selected provider's named env_key for API-key authentication; login, auth helpers and selected CLI profiles are not evaluated here."), nil
}

func (p *Pi) ProviderEnvironment() ([]ProviderDiagnostic, error) {
	profile, active, err := p.ReadProvider()
	if err != nil || !active || profile.EnvKey == "" {
		return nil, err
	}
	var auth map[string]struct {
		Type string `json:"type"`
	}
	// Only credential type metadata is decoded, never keys or OAuth tokens.
	if err := decodeJSONFile(filepath.Join(p.agentDir(), "auth.json"), &auth); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if entry := auth[profile.Name]; entry.Type == "api_key" || entry.Type == "oauth" {
		return nil, nil // Stored authentication has priority over models.json.
	}
	return missingProviderEnvironment(profile.EnvKey, p.ProviderFile()+" (selected provider apiKey)",
		"Pi resolves explicit $NAME and ${NAME} references in models.json after runtime and stored credentials. Runtime flags and extensions are not evaluated here."), nil
}

func (c *Crush) ProviderEnvironment() ([]ProviderDiagnostic, error) {
	block, err := c.readCrushBlock("provider")
	if err != nil || len(block) == 0 {
		return nil, err // Arbitrary crushrc and legacy JSON evaluation are out of scope.
	}
	p, active, err := c.ReadProvider()
	if err != nil || !active {
		return nil, err
	}
	return missingProviderEnvironment(p.EnvKey, c.ProviderFile()+" (managed provider --api-key)",
		"Crush expands the managed reference while loading crushrc. Native scripts can supply it later; their effective environment is unknown because lazyagents never executes them."), nil
}

// Explicit references only: escaped dollars, commands and malformed references
// must never become variable names in public DTOs.
func providerEnvReference(value string) string {
	name, ok := strings.CutPrefix(value, "$")
	if !ok {
		return ""
	}
	if strings.HasPrefix(name, "{") && strings.HasSuffix(name, "}") {
		name = name[1 : len(name)-1]
	}
	if !envNameRe.MatchString(name) {
		return ""
	}
	return name
}
