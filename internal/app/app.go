// Package app is the composition root: it loads paths and config (Load),
// builds the registered modules and exposes the TUI and CLI. Only main
// imports it. A new module = a package in internal/modules/<name> with its
// Feature and ONE line in features().
package app

import (
	"fmt"
	"os"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// App is the assembled lazyagents: shared deps and registered modules.
type App struct {
	Deps     *feature.Deps
	features []feature.Feature
}

// Load boots the app: XDG paths, config.yaml, adapters and modules.
func Load(version string) (*App, error) {
	paths, err := core.DefaultPaths()
	if err != nil {
		return nil, err
	}
	return LoadWith(paths, version)
}

// LoadWith is Load with injected paths (tests).
func LoadWith(paths core.Paths, version string) (*App, error) {
	d := &feature.Deps{Version: version}
	if migrated, err := core.MigrateConfig(paths); err != nil {
		d.Notice(err.Error())
	} else if migrated {
		d.Notice("config.json migrated to " + paths.ConfigPath())
	}
	paths = paths.WithConfig() // config.yaml overrides (libraryDir)
	cfg, err := core.ReadConfig(paths.ConfigPath())
	if err != nil {
		d.Notice(err.Error() + "; using defaults") // an invalid config never blocks boot
	}
	d.Paths, d.Config, d.Adapters = paths, cfg, agent.AllWithIndex(paths.Home, paths.TranscriptIndexPath())

	a := &App{Deps: d, features: features()}
	a.reserveNames()
	return a, nil
}

// reserveNames marks built-in tab and command names so runtime tabs
// (plugins) cannot collide; that is why plugins is last in the registry.
func (a *App) reserveNames() {
	a.Deps.Reserve("doctor", "help", "tui") // tui: is the layout config section
	for _, f := range a.features {
		a.Deps.Reserve(f.Name)
		if f.Commands == nil {
			continue
		}
		for _, c := range f.Commands(a.Deps) {
			a.Deps.Reserve(c.Name)
		}
	}
}

// Modules returns the visible tabs with the config layout applied (see Layout).
func (a *App) Modules() []module.Module {
	mods, _ := a.Layout()
	return mods
}

// instantiate builds tabs in default order: registry order, with Last
// features at the end, after runtime tabs too.
func (a *App) instantiate() []module.Module {
	var mods, last []module.Module
	for _, f := range a.features {
		if f.Tabs == nil {
			continue
		}
		if f.Last {
			last = append(last, f.Tabs(a.Deps)...)
			continue
		}
		mods = append(mods, f.Tabs(a.Deps)...)
	}
	return append(mods, last...)
}

// Commands gathers every module subcommand plus the aggregated doctor.
func (a *App) Commands() []cli.Command {
	var cmds []cli.Command
	var checks []cli.Check
	for _, f := range a.features {
		if f.Commands != nil {
			cmds = append(cmds, f.Commands(a.Deps)...)
		}
		if f.Checks != nil {
			checks = append(checks, f.Checks(a.Deps)...)
		}
	}
	return append(cmds, cli.DoctorCommand(checks))
}

// RunCLI runs a headless subcommand and returns the exit code.
func (a *App) RunCLI(args []string) int {
	for _, n := range a.Deps.Notices() {
		fmt.Fprintln(os.Stderr, "lazyagents:", n)
	}
	ids := make([]string, 0, len(a.Deps.Adapters))
	for _, ad := range a.Deps.Adapters {
		ids = append(ids, ad.ID())
	}
	c := cli.Context{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Paths: a.Deps.Paths, Agents: a.Deps.Agents, AgentIDs: ids}
	return cli.Run(args, c, a.Commands())
}

// Close releases module resources (plugin processes); call on exit.
func (a *App) Close() {
	for _, f := range a.features {
		if f.Close != nil {
			f.Close()
		}
	}
}

// FeatureNames lists the registered module ids, in registry order (docs tests
// check each one has a guide).
func (a *App) FeatureNames() []string {
	names := make([]string, 0, len(a.features))
	for _, f := range a.features {
		names = append(names, f.Name)
	}
	return names
}
