package app

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins"
	"github.com/rogeriojunior31/lazyagents/internal/modules/providers"
	"github.com/rogeriojunior31/lazyagents/internal/session"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/agents"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/sessions"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/skills"
	usagemod "github.com/rogeriojunior31/lazyagents/internal/tui/modules/usage"
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
		plugins.Feature(),
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
