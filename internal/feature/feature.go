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

	mu        sync.Mutex
	notices   []string
	reserved  map[string]bool
	hidden    map[string]bool // tabs hidden by the tui: config section
	skipped   map[string]bool // hidden tabs some feature asked about
	detectMu  sync.Mutex
	detection agent.Detection
}

// Agents returns the agent detection, memoized: Detect runs each CLI's
// --version, so cached readers reuse it until an explicit refresh.
func (d *Deps) Agents() []agent.Agent {
	return d.DetectAgents(false).Agents
}

// DetectAgents serializes scans; cached readers keep the last completed result
// while a refresh probes CLIs. Copies prevent callers from mutating the cache.
func (d *Deps) DetectAgents(refresh bool) agent.Detection {
	d.mu.Lock()
	previous := cloneDetection(d.detection)
	d.mu.Unlock()
	if !refresh && previous.Revision != 0 {
		return previous
	}
	d.detectMu.Lock()
	defer d.detectMu.Unlock()
	d.mu.Lock()
	previous = cloneDetection(d.detection)
	d.mu.Unlock()
	if !refresh && previous.Revision != 0 {
		return previous
	}
	next := agent.Detection{Agents: agent.DetectAll(d.Adapters), Revision: previous.Revision + 1}
	for i, current := range next.Agents {
		if current.ID != d.Adapters[i].ID() {
			current.ID = d.Adapters[i].ID()
			current.DetectionError = "Detection returned an invalid agent; check the installation and refresh Agents."
		}
		if current.DetectionError != "" {
			for _, old := range previous.Agents {
				if old.ID == current.ID {
					warning := current.DetectionError
					current = old
					current.DetectionError = warning
					break
				}
			}
		}
		next.Agents[i] = current
	}
	d.mu.Lock()
	d.detection = cloneDetection(next)
	d.mu.Unlock()
	return cloneDetection(next)
}

func cloneDetection(snapshot agent.Detection) agent.Detection {
	snapshot.Agents = append([]agent.Agent(nil), snapshot.Agents...)
	for i := range snapshot.Agents {
		snapshot.Agents[i].ReadDirs = append([]string(nil), snapshot.Agents[i].ReadDirs...)
	}
	return snapshot
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
