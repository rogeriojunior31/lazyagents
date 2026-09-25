package agent

import "path/filepath"

// Claude Code env keys a provider profile controls, read from settings.json "env".
const (
	claudeEnvBaseURL = "ANTHROPIC_BASE_URL"
	claudeEnvToken   = "ANTHROPIC_AUTH_TOKEN"
	claudeEnvModel   = "ANTHROPIC_MODEL"
)

func (c *Claude) ProviderFile() string { return filepath.Join(c.configDir(), "settings.json") }

// claudeEnv reads settings.json "env" as a string map. Non-string values (the
// file is the user's) are ignored, not errors.
func claudeEnv(s *settings) (map[string]string, error) {
	env := map[string]string{}
	var raw map[string]any
	ok, err := s.get("env", &raw)
	if err != nil || !ok {
		return env, err
	}
	for k, v := range raw {
		if str, ok := v.(string); ok {
			env[k] = str
		}
	}
	return env, nil
}

func (c *Claude) ReadProvider() (ProviderProfile, bool, error) {
	s, err := readSettings(c.ProviderFile())
	if err != nil {
		return ProviderProfile{}, false, err
	}
	env, err := claudeEnv(s)
	if err != nil {
		return ProviderProfile{}, false, err
	}
	p := ProviderProfile{
		BaseURL:  env[claudeEnvBaseURL],
		Model:    env[claudeEnvModel],
		HasToken: env[claudeEnvToken] != "",
	}
	if p.BaseURL == "" && p.Model == "" && !p.HasToken {
		return ProviderProfile{}, false, nil
	}
	return p, true, nil
}

func (c *Claude) ApplyProvider(p ProviderProfile, backupsDir string) error {
	return c.writeEnv(backupsDir, map[string]string{
		claudeEnvBaseURL: p.BaseURL,
		claudeEnvToken:   p.Token,
		claudeEnvModel:   p.Model,
	})
}

func (c *Claude) ClearProvider(backupsDir string) error {
	return c.writeEnv(backupsDir, map[string]string{})
}

// writeEnv sets values in "env" (an empty value removes the key). Only the
// provider keys are touched; an emptied "env" is removed.
func (c *Claude) writeEnv(backupsDir string, values map[string]string) error {
	path := c.ProviderFile()
	s, err := readSettings(path)
	if err != nil {
		return err
	}
	env := map[string]any{}
	if _, err := s.get("env", &env); err != nil {
		return err
	}
	if env == nil {
		env = map[string]any{}
	}
	for _, k := range []string{claudeEnvBaseURL, claudeEnvToken, claudeEnvModel} {
		if v := values[k]; v != "" {
			env[k] = v
		} else {
			delete(env, k)
		}
	}
	var set any = env
	if len(env) == 0 {
		set = nil
	}
	if err := s.set("env", set); err != nil {
		return err
	}
	if values[claudeEnvToken] != "" {
		s.perm = 0o600
	}
	return s.save(backupsDir)
}
