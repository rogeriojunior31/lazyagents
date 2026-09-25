package agent

// Claude Code keeps hooks in settings.json under "hooks". There is no trust
// state: whatever is in the file runs.

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
