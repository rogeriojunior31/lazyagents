package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Hook events. Claude Code and Codex use the same CamelCase names in their
// config; each adapter's HookEvents lists the ones it actually fires.
const (
	HookSessionStart     = "SessionStart"
	HookSessionEnd       = "SessionEnd"
	HookUserPromptSubmit = "UserPromptSubmit"
	HookPreToolUse       = "PreToolUse"
	HookPostToolUse      = "PostToolUse"
	HookPreCompact       = "PreCompact"
	HookNotification     = "Notification"
	HookStop             = "Stop"
	HookSubagentStop     = "SubagentStop"
	hookCommandType      = "command"
	hookGroupsKey        = "hooks"
)

// Hook is a command fired by an agent event. Its identity is (Event, Matcher,
// Command): that is how lazyagents finds its own hooks without touching others.
type Hook struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"` // event filter ("" = all)
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds; 0 = agent default
	// Async runs the hook without blocking the turn. Not part of the identity,
	// but must be kept: dropping it changes behavior.
	Async bool `json:"async,omitempty"`
}

// Same reports whether two hooks are the same for installation purposes.
func (h Hook) Same(o Hook) bool {
	return sameEvent(h.Event, o.Event) && h.Matcher == o.Matcher && h.Command == o.Command
}

// sameEvent compares event names ignoring case and separators: Codex writes
// CamelCase but uses snake_case internally, and hand-written files have either.
func sameEvent(a, b string) bool { return normEvent(a) == normEvent(b) }

func normEvent(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

// HooksHost is implemented by adapters that can read and write hooks.
// Optional, by type assertion.
type HooksHost interface {
	// HookEvents lists the events this agent fires.
	HookEvents() []string
	// HooksFile is the file AddHook/RemoveHook write.
	HooksFile() string
	// ReadHooks returns ALL configured hooks, including foreign ones.
	ReadHooks() ([]Hook, error)
	// AddHook installs the hook (idempotent), backing the live file up.
	AddHook(h Hook, backupsDir string) error
	// RemoveHook uninstalls by identity; a missing hook is a no-op.
	RemoveHook(h Hook, backupsDir string) error
	// HooksNote is a short warning about the agent state ("" = none), e.g.
	// hooks turned off in config or a pending confirmation.
	HooksNote() string
}

// hookGroup is one event's hook group as both CLIs write it: an optional
// matcher and the command list.
type hookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	Async   bool   `json:"async,omitempty"`
}

// hookDoc is a JSON file with a hooks map under "hooks" (Claude Code's
// settings.json, Codex's hooks.json). Groups stay raw and are rewritten only
// when they contain the hook being changed: foreign groups go back to disk
// byte for byte, with fields lazyagents does not know.
type hookDoc struct {
	file  *settings
	byEv  *object // event → list of groups
	order []string
}

func openHookDoc(path string) (*hookDoc, error) {
	s, err := readSettings(path)
	if err != nil {
		return nil, err
	}
	d := &hookDoc{file: s, byEv: &object{}}
	if raw, ok := s.raw(hookGroupsKey); ok {
		o, err := decodeObject(raw)
		if err != nil {
			return nil, fmt.Errorf("reading %s: key %q: %w", path, hookGroupsKey, err)
		}
		d.byEv = o
	}
	d.order = d.byEv.keys()
	return d, nil
}

// eventKey finds the event key as spelled in the file; matching
// case-insensitively avoids creating a duplicate key.
func (d *hookDoc) eventKey(event string) string {
	for _, k := range d.order {
		if sameEvent(k, event) {
			return k
		}
	}
	return event
}

func (d *hookDoc) groups(event string) []json.RawMessage {
	var groups []json.RawMessage
	if _, err := d.byEv.get(d.eventKey(event), &groups); err != nil {
		return nil
	}
	return groups
}

// list returns every hook in the file. Entries whose type is not "command" are
// skipped on read (and kept on write).
func (d *hookDoc) list() []Hook {
	var out []Hook
	for _, event := range d.byEv.keys() {
		for _, raw := range d.groups(event) {
			var g hookGroup
			if err := json.Unmarshal(raw, &g); err != nil {
				continue
			}
			for _, e := range g.Hooks {
				if e.Type != "" && e.Type != hookCommandType {
					continue
				}
				out = append(out, Hook{Event: event, Matcher: g.Matcher, Command: e.Command, Timeout: e.Timeout, Async: e.Async})
			}
		}
	}
	return out
}

// add installs the hook as a new group at the end of the event. Existing
// groups are never edited, so no unknown field is lost. false: already there.
func (d *hookDoc) add(h Hook) (bool, error) {
	for _, existing := range d.list() {
		if existing.Same(h) {
			return false, nil
		}
	}
	group, err := json.Marshal(hookGroup{
		Matcher: h.Matcher,
		Hooks:   []hookEntry{{Type: hookCommandType, Command: h.Command, Timeout: h.Timeout, Async: h.Async}},
	})
	if err != nil {
		return false, err
	}
	key := d.eventKey(h.Event)
	groups := append(d.groups(h.Event), group)
	if err := d.byEv.set(key, groups); err != nil {
		return false, err
	}
	d.order = d.byEv.keys()
	return true, nil
}

// remove drops the hook. A group holding only it goes away; a group shared
// with other commands is rewritten without it (the only case a foreign group
// is rewritten). false: nothing to remove.
func (d *hookDoc) remove(h Hook) (bool, error) {
	key := d.eventKey(h.Event)
	groups := d.groups(h.Event)
	kept := make([]json.RawMessage, 0, len(groups))
	changed := false
	for _, raw := range groups {
		var g hookGroup
		if err := json.Unmarshal(raw, &g); err != nil || g.Matcher != h.Matcher {
			kept = append(kept, raw)
			continue
		}
		rest := make([]hookEntry, 0, len(g.Hooks))
		for _, e := range g.Hooks {
			if e.Command == h.Command && (e.Type == "" || e.Type == hookCommandType) {
				changed = true
				continue
			}
			rest = append(rest, e)
		}
		switch {
		case len(rest) == len(g.Hooks):
			kept = append(kept, raw)
		case len(rest) > 0:
			g.Hooks = rest
			redone, err := json.Marshal(g)
			if err != nil {
				return false, err
			}
			kept = append(kept, redone)
		}
	}
	if !changed {
		return false, nil
	}
	if len(kept) == 0 {
		d.byEv.delete(key)
	} else if err := d.byEv.set(key, kept); err != nil {
		return false, err
	}
	d.order = d.byEv.keys()
	return true, nil
}

// save writes the file with a backup. An empty hooks map is removed.
func (d *hookDoc) save(backupsDir string) error {
	if d.byEv.empty() {
		d.file.delete(hookGroupsKey)
	} else if err := d.file.set(hookGroupsKey, d.byEv); err != nil {
		return err
	}
	return d.file.save(backupsDir)
}

// ReadHookFile reads any file in the hooks format (Claude Code settings.json,
// Codex hooks.json, a plugin's hooks/hooks.json), so other modules can import
// hooks without knowing the format.
func ReadHookFile(path string) ([]Hook, error) { return hookList(path) }

// hookAdd and hookRemove are shared by adapters whose file keeps the hooks map
// under "hooks".
func hookAdd(path string, h Hook, backupsDir string) error {
	d, err := openHookDoc(path)
	if err != nil {
		return err
	}
	changed, err := d.add(h)
	if err != nil || !changed {
		return err
	}
	return d.save(backupsDir)
}

func hookRemove(path string, h Hook, backupsDir string) error {
	d, err := openHookDoc(path)
	if err != nil {
		return err
	}
	changed, err := d.remove(h)
	if err != nil || !changed {
		return err
	}
	return d.save(backupsDir)
}

func hookList(path string) ([]Hook, error) {
	d, err := openHookDoc(path)
	if err != nil {
		return nil, err
	}
	return d.list(), nil
}
