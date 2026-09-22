package agent

// O Claude Code guarda os hooks no próprio settings.json, na chave "hooks".
// Não há estado de confiança: o que está no arquivo roda.

func (c *Claude) HookEvents() []string {
	return []string{
		HookSessionStart, HookUserPromptSubmit, HookPreToolUse, HookPostToolUse,
		HookNotification, HookStop, HookSubagentStop, HookPreCompact, HookSessionEnd,
	}
}

func (c *Claude) HooksFile() string { return c.ProviderFile() } // ~/.claude/settings.json

func (c *Claude) ReadHooks() ([]Hook, error) { return hookList(c.HooksFile()) }

func (c *Claude) AddHook(h Hook, backupsDir string) error {
	return hookAdd(c.HooksFile(), h, backupsDir)
}

func (c *Claude) RemoveHook(h Hook, backupsDir string) error {
	return hookRemove(c.HooksFile(), h, backupsDir)
}

func (c *Claude) HooksNote() string { return "" }
