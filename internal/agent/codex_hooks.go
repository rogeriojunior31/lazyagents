package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// Codex keeps hooks in ~/.codex/hooks.json (same shape as Claude Code's
// settings.json) and, in config.toml, two things lazyagents does NOT write:
// [features] hooks = true, and [hooks.state."<file>:<event>:<i>:<j>"]
// trusted_hash, the record that the user trusted that command. Forging the hash
// would approve running a command on the user's behalf, so a hook installed
// here only runs after Codex itself asks (see HooksNote).

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

// HooksNote tells what is missing for a hook to actually run in Codex.
func (c *Codex) HooksNote() string {
	if !c.hooksEnabled() {
		return "hooks are off in Codex: set hooks = true under [features] in config.toml"
	}
	return "a new hook only runs after you confirm trust in Codex itself"
}

// hooksEnabled reads [features] hooks from config.toml.
func (c *Codex) hooksEnabled() bool {
	data, err := os.ReadFile(c.ProviderFile())
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
