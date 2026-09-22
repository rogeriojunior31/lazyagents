package app

import (
	"encoding/json"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/modules/providers"
	"github.com/rogeriojunior31/lazyagents/internal/plugin"
	"github.com/rogeriojunior31/lazyagents/internal/session"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/agents"
	pluginmod "github.com/rogeriojunior31/lazyagents/internal/tui/modules/plugin"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/sessions"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/skills"
	usagemod "github.com/rogeriojunior31/lazyagents/internal/tui/modules/usage"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
	"github.com/rogeriojunior31/lazyagents/internal/usage"
)

// features é O registro. A ordem é a ordem das abas e do help da CLI; o
// módulo de plugins vem por último porque descobre ids em runtime e precisa
// dos nomes embutidos já reservados.
func features() []feature.Feature {
	return []feature.Feature{
		skillsFeature(),
		sessionsFeature(),
		agentsFeature(),
		providers.Feature(),
		usageFeature(),
		pluginsFeature(),
	}
}

// --- features ainda não migradas para internal/modules/<nome> ---

func skillsFeature() feature.Feature {
	var svc *skill.Service
	get := func(d *feature.Deps) *skill.Service {
		if svc == nil {
			svc = skill.New(d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "skills",
		Tabs: func(d *feature.Deps) []module.Module {
			m := skills.NewSkills(get(d))
			return []module.Module{&m}
		},
		Commands: func(d *feature.Deps) []cli.Command { return cli.SkillCommands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return cli.SkillChecks(get(d)) },
	}
}

func sessionsFeature() feature.Feature {
	var svc *session.Service
	get := func(d *feature.Deps) *session.Service {
		if svc == nil {
			svc = session.New(d.Adapters, d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "sessions",
		Tabs: func(d *feature.Deps) []module.Module {
			m := sessions.NewSessions(get(d), d.Paths.Home)
			return []module.Module{&m}
		},
		Commands: func(d *feature.Deps) []cli.Command { return cli.SessionCommands(get(d)) },
	}
}

func agentsFeature() feature.Feature {
	return feature.Feature{
		Name: "agents",
		Tabs: func(d *feature.Deps) []module.Module {
			m := agents.NewAgents()
			return []module.Module{&m}
		},
	}
}

func usageFeature() feature.Feature {
	var svc *usage.Service
	get := func(d *feature.Deps) *usage.Service {
		if svc == nil {
			svc = usage.New(d.Adapters, d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "usage",
		Last: true,
		Tabs: func(d *feature.Deps) []module.Module {
			m := usagemod.NewUsage(get(d))
			return []module.Module{&m}
		},
		Commands: func(d *feature.Deps) []cli.Command { return cli.UsageCommands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return cli.UsageChecks(get(d)) },
	}
}

// pluginsFeature descobre os binários em <ConfigDir>/plugins e transforma
// cada um numa aba, um subcomando e uma seção do doctor.
func pluginsFeature() feature.Feature {
	var svc *plugin.Service
	var found []plugin.Plugin
	var warnings []string
	get := func(d *feature.Deps) *plugin.Service {
		if svc != nil {
			return svc
		}
		svc = plugin.New(d.Paths)
		pls, warns := svc.List()
		for _, pl := range pls {
			if d.Reserved(pl.ID) {
				warns = append(warns, "plugin "+pl.ID+" ignorado: id reservado por uma aba ou comando embutido")
				continue
			}
			found = append(found, pl)
		}
		warnings = warns
		d.Notice(warns...)
		return svc
	}
	initFor := func(d *feature.Deps) func(plugin.Plugin) plugin.Msg {
		return func(pl plugin.Plugin) plugin.Msg { return pluginInit(d, pl) }
	}
	return feature.Feature{
		Name: "plugins",
		Tabs: func(d *feature.Deps) []module.Module {
			s := get(d)
			mods := make([]module.Module, 0, len(found))
			for _, pl := range found {
				mods = append(mods, pluginmod.New(s, pl, pluginInit(d, pl)))
			}
			return mods
		},
		Commands: func(d *feature.Deps) []cli.Command { return cli.PluginCommands(get(d), found) },
		Checks: func(d *feature.Deps) []cli.Check {
			return cli.PluginChecks(get(d), found, initFor(d), warnings)
		},
		Close: func() {
			if svc != nil {
				svc.Close()
			}
		},
	}
}

// pluginInit monta a mensagem init de um plugin: paths, tema ativo e a seção
// <id>: do config.yaml como JSON.
func pluginInit(d *feature.Deps, pl plugin.Plugin) plugin.Msg {
	m := plugin.Msg{
		Home: d.Paths.Home, ConfigDir: d.Paths.ConfigDir, DataDir: d.Paths.DataDir, LibraryDir: d.Paths.LibraryDir(),
		Theme: &plugin.Theme{ID: theme.Current()},
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
