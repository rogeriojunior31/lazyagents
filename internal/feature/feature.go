// Package feature is the contract between a module and the composition root.
// A module lives in one package (internal/modules/<name>); app only knows the
// line that registers it, and neither cli nor tui knows any module.
package feature

import (
	"sync"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Deps is what every module receives. A module builds its own service inside
// Feature, so Deps does not grow with each new module.
type Deps struct {
	Paths    core.Paths
	Adapters []agent.Adapter
	Version  string
	// Config is config.yaml read at boot (zero if missing or invalid); each
	// module reads its section with Config.Section("<id>", &cfg).
	Config core.Config

	mu         sync.Mutex
	notices    []string
	reserved   map[string]bool
	hidden     map[string]bool // tabs hidden by the tui: config section
	skipped    map[string]bool // hidden tabs some feature asked about
	detectOnce sync.Once
	agents     []agent.Agent
}

// Agents returns the agent detection, memoized: Detect runs each CLI's
// --version, so it runs once and only on demand.
func (d *Deps) Agents() []agent.Agent {
	d.detectOnce.Do(func() { d.agents = agent.DetectAll(d.Adapters) })
	return d.agents
}

// Notice records a boot notice (migration, invalid config, skipped plugin).
// The TUI prints it after leaving the alt screen; the CLI prints it at once.
func (d *Deps) Notice(msgs ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notices = append(d.notices, msgs...)
}

// Notices returns the accumulated notices.
func (d *Deps) Notices() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.notices...)
}

// Reserve marks names taken by built-in tabs and commands; runtime tabs
// (plugins) check Reserved before registering.
func (d *Deps) Reserve(names ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reserved == nil {
		d.reserved = map[string]bool{}
	}
	for _, n := range names {
		d.reserved[n] = true
	}
}

// Reserved reports whether name belongs to a built-in tab or command.
func (d *Deps) Reserved(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reserved[name]
}

// HideTabs marks tabs hidden by config. The app calls it before building
// the tabs.
func (d *Deps) HideTabs(ids ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.hidden == nil {
		d.hidden = map[string]bool{}
	}
	for _, id := range ids {
		d.hidden[id] = true
	}
}

// TabHidden reports whether tab id is hidden. Built-in hidden tabs are still
// created (they feed others through events); features that spawn a process
// per tab (plugins) check this and skip the tab.
func (d *Deps) TabHidden(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.hidden[id] {
		return false
	}
	if d.skipped == nil {
		d.skipped = map[string]bool{}
	}
	d.skipped[id] = true
	return true
}

// SkippedTabs returns the hidden tabs some feature did not create.
func (d *Deps) SkippedTabs() map[string]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]bool{}
	for id := range d.skipped {
		out[id] = true
	}
	return out
}

// Feature is what a module declares; every field is optional (a module can
// be CLI-only or tab-only).
type Feature struct {
	// Name is the module id; it becomes a name reserved from plugins.
	Name string
	// Tabs returns the module tabs (usually one; plugins return one per binary).
	Tabs func(d *Deps) []module.Module
	// Commands are the CLI subcommands.
	Commands func(d *Deps) []cli.Command
	// Checks are the doctor sections.
	Checks func(d *Deps) []cli.Check
	// Close releases module resources on exit (plugin processes).
	Close func()
	// Last pushes the tab to the end, after plugin tabs: for read-only tabs
	// (Usage, Agents) that should not compete with the working ones.
	Last bool
}
