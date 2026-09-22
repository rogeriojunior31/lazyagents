package sessions

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registra o módulo de sessões: a aba e o comando `sessions`.
func Feature() feature.Feature {
	var svc *Service
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Adapters, d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "sessions",
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(get(d), d.Paths.Home)
			return []module.Module{&t}
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d)) },
	}
}
