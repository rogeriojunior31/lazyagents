package agent

import "path/filepath"

// Chaves de env do Claude Code que o perfil de provedor controla. O CLI lê
// essas variáveis do bloco "env" do settings.json.
const (
	claudeEnvBaseURL = "ANTHROPIC_BASE_URL"
	claudeEnvToken   = "ANTHROPIC_AUTH_TOKEN"
	claudeEnvModel   = "ANTHROPIC_MODEL"
)

// ProviderFile é o settings.json do usuário.
func (c *Claude) ProviderFile() string { return filepath.Join(c.configDir(), "settings.json") }

// claudeEnv lê o bloco "env" do settings.json como mapa de strings. Valores
// não-string (o arquivo é do usuário) são ignorados em vez de virarem erro.
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

// writeEnv aplica os valores em "env": chave com valor entra, chave vazia
// sai. Só as três chaves do provedor são tocadas — o resto do env (e do
// arquivo) sobrevive intacto. "env" vazio é removido para não deixar lixo.
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
