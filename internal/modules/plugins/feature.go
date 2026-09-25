package plugins

import (
	"encoding/json"
	"fmt"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Feature registers external plugins: each binary in <ConfigDir>/plugins becomes
// a tab, a subcommand and a doctor line. It discovers ids at runtime, so it is
// last in the registry: built-in names are already reserved when it runs.
func Feature() feature.Feature {
	var svc *Service
	var found []Plugin
	var warnings []string
	get := func(d *feature.Deps) *Service {
		if svc != nil {
			return svc
		}
		svc = New(d.Paths)
		pls, warns := svc.List()
		for _, pl := range pls {
			if d.Reserved(pl.ID) {
				warns = append(warns, fmt.Sprintf("plugin %s ignored: id reserved by a built-in tab or command", pl.ID))
				continue
			}
			found = append(found, pl)
		}
		warnings = warns
		d.Notice(warns...)
		return svc
	}
	return feature.Feature{
		Name: "plugins",
		Tabs: func(d *feature.Deps) []module.Module {
			s := get(d)
			mods := make([]module.Module, 0, len(found))
			for _, pl := range found {
				if d.TabHidden(pl.ID) {
					continue // hidden by config: do not spawn the process
				}
				mods = append(mods, newTab(s, pl, initMsg(d, pl)))
			}
			return mods
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d), found) },
		Checks: func(d *feature.Deps) []cli.Check {
			s := get(d)
			return checks(s, found, func(pl Plugin) Msg { return initMsg(d, pl) }, warnings)
		},
		Close: func() {
			if svc != nil {
				svc.Close()
			}
		},
	}
}

// initMsg builds a plugin's init: paths, active theme and its <id>: config section as JSON.
func initMsg(d *feature.Deps, pl Plugin) Msg {
	m := Msg{
		Home: d.Paths.Home, ConfigDir: d.Paths.ConfigDir, DataDir: d.Paths.DataDir, LibraryDir: d.Paths.LibraryDir(),
		Theme: &Theme{ID: theme.Current()},
	}
	for _, p := range theme.Options() {
		if p.ID == theme.Current() {
			m.Theme.Colors = p.Colors
		}
	}
	var section any
	if err := d.Config.Section(pl.ID, &section); err == nil && section != nil {
		m.Config, _ = json.Marshal(section)
	}
	return m
}
