package hooks

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registra o módulo de hooks: aba, comandos e a seção do doctor.
func Feature() feature.Feature {
	var svc *Service
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Adapters, d.Paths)
			svc.Detect = d.Agents // reusa a detecção memoizada
		}
		return svc
	}
	return feature.Feature{
		Name: "hooks",
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(get(d))
			return []module.Module{&t}
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return checks(get(d)) },
	}
}
