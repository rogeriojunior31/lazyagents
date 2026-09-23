package usage

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registra o módulo de uso. Last: a aba é de consulta e fica sempre
// no fim da barra, depois até das abas de plugin.
func Feature() feature.Feature {
	var svc *Service
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Adapters, d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "usage",
		Last: true,
		Tabs: func(d *feature.Deps) []module.Module {
			var cfg config
			if err := d.Config.Section("usage", &cfg); err != nil {
				d.Notice(err.Error() + "; usando os filtros padrão")
			}
			t := newTab(get(d), cfg)
			return []module.Module{&t}
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return checks(get(d)) },
	}
}
