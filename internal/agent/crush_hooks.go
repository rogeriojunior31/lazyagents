package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

// Crush fires only PreToolUse (crush 0.96.1). lazyagents installs its hooks in
// the "hooks" block of the global crushrc, each named lazyagents-<hash> so it
// is removed alone; Crush reads a hook's output in Claude Code's format too.

func (c *Crush) HookEvents() []string { return []string{HookPreToolUse} }

func (c *Crush) HooksFile() string { return c.crushrcFile() }

func (c *Crush) HooksNote() string { return "" }

// ReadHooks lists lazyagents' hooks and those in crush.json. Hooks a crushrc
// defines outside the block are Bash that only running it would reveal, so
// they are not listed.
func (c *Crush) ReadHooks() ([]Hook, error) {
	block, err := c.readCrushBlock("hooks")
	if err != nil {
		return nil, err
	}
	hooks := crushBlockHooks(block)
	var cfg struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	}
	if decodeJSONFile(c.configFile(), &cfg) == nil {
		for ev, list := range cfg.Hooks {
			for _, h := range list {
				hooks = append(hooks, Hook{Event: ev, Matcher: h.Matcher, Command: h.Command, Timeout: h.Timeout})
			}
		}
	}
	return hooks, nil
}

func (c *Crush) AddHook(h Hook, backupsDir string) error {
	block, err := c.readCrushBlock("hooks")
	if err != nil {
		return err
	}
	hooks := crushBlockHooks(block)
	if slices.ContainsFunc(hooks, h.Same) {
		return nil
	}
	return c.writeCrushBlock("hooks", crushHookLines(append(hooks, h)), false, backupsDir)
}

func (c *Crush) RemoveHook(h Hook, backupsDir string) error {
	block, err := c.readCrushBlock("hooks")
	if err != nil {
		return err
	}
	hooks := crushBlockHooks(block)
	kept := slices.DeleteFunc(slices.Clone(hooks), h.Same)
	if len(kept) == len(hooks) {
		return nil
	}
	return c.writeCrushBlock("hooks", crushHookLines(kept), false, backupsDir)
}

func crushBlockHooks(block []string) []Hook {
	var hooks []Hook
	for _, line := range block {
		w := shWords(line)
		if len(w) < 3 || w[0] != "hook" || w[1] != "add" {
			continue
		}
		h := Hook{Event: w[2]}
		h.Command, _ = shFlag(w, "--command")
		h.Matcher, _ = shFlag(w, "--matcher")
		if t, ok := shFlag(w, "--timeout"); ok {
			h.Timeout, _ = strconv.Atoi(t)
		}
		hooks = append(hooks, h)
	}
	return hooks
}

// crushHookLines writes each hook as a `hook add` line. The name comes from
// the hook's identity, so the same hook always gets the same name.
func crushHookLines(hooks []Hook) []string {
	lines := make([]string, 0, len(hooks))
	for _, h := range hooks {
		sum := sha256.Sum256([]byte(normEvent(h.Event) + "\x00" + h.Matcher + "\x00" + h.Command))
		parts := []string{"hook add", h.Event, "--command", shQuote(h.Command), "--name", "lazyagents-" + hex.EncodeToString(sum[:4])}
		if h.Matcher != "" {
			parts = append(parts, "--matcher", shQuote(h.Matcher))
		}
		if h.Timeout > 0 {
			parts = append(parts, "--timeout", strconv.Itoa(h.Timeout))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return lines
}
