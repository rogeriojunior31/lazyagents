package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// O Codex guarda os hooks num arquivo próprio (~/.codex/hooks.json, mesma
// forma do settings.json do Claude Code) e mantém no config.toml duas coisas
// que o lazyagents NÃO escreve:
//
//   - [features] hooks = true, que liga o recurso;
//   - [hooks.state."<arquivo>:<evento>:<i>:<j>"] trusted_hash, o registro de
//     que o usuário confiou naquele comando.
//
// O trusted_hash é justamente a confirmação de que o usuário aceitou rodar
// aquele comando. Forjá-lo seria aprovar execução de comando em nome dele,
// então hook instalado aqui só passa a valer depois que o próprio Codex
// perguntar (ver HooksNote).

func (c *Codex) HookEvents() []string {
	return []string{
		HookSessionStart, HookUserPromptSubmit, HookPreToolUse, HookPostToolUse,
		HookPreCompact, HookSessionEnd,
	}
}

func (c *Codex) HooksFile() string { return filepath.Join(c.configDir(), "hooks.json") }

func (c *Codex) ReadHooks() ([]Hook, error) { return hookList(c.HooksFile()) }

func (c *Codex) AddHook(h Hook, backupsDir string) error {
	return hookAdd(c.HooksFile(), h, backupsDir)
}

func (c *Codex) RemoveHook(h Hook, backupsDir string) error {
	return hookRemove(c.HooksFile(), h, backupsDir)
}

// HooksNote avisa o que falta para um hook realmente rodar no Codex.
func (c *Codex) HooksNote() string {
	if !c.hooksEnabled() {
		return "hooks are off in Codex: set hooks = true under [features] in config.toml"
	}
	return "a new hook only runs after you confirm trust in Codex itself"
}

// hooksEnabled lê [features] hooks do config.toml.
func (c *Codex) hooksEnabled() bool {
	data, err := os.ReadFile(c.ProviderFile()) // ~/.codex/config.toml
	if err != nil {
		return false
	}
	table := ""
	for _, raw := range splitLines(string(data)) {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			table = strings.Trim(line, "[]")
			continue
		}
		if table != "features" {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && strings.TrimSpace(key) == "hooks" {
			return strings.TrimSpace(value) == "true"
		}
	}
	return false
}
