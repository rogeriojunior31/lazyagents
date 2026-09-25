package skills

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registers the skills module: tab, library commands and the doctor
// section that validates each SKILL.md.
func Feature() feature.Feature {
	var svc *Service
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "skills",
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(get(d))
			return []module.Module{&t}
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return checks(get(d)) },
	}
}
