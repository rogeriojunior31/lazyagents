package usage

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"time"
)

// Feature registers the usage module. Last: it is a read-only tab and always
// sits at the end of the bar, after plugin tabs too.
func Feature() feature.Feature {
	var svc *Service
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Adapters, d.Paths)
			var cfg config
			if err := d.Config.Section("usage", &cfg); err != nil {
				svc.PriceError = "Invalid usage configuration; configured fallback prices are disabled"
			} else if err := agent.ValidatePriceOverrides(cfg.Prices, time.Now()); err != nil {
				svc.PriceError = err.Error()
			} else {
				svc.Prices = cfg.Prices
			}
			if svc.PriceError != "" {
				d.Notice(svc.PriceError)
			}
		}
		return svc
	}
	return feature.Feature{
		Name: "usage",
		Last: true,
		Tabs: func(d *feature.Deps) []module.Module {
			var cfg config
			if err := d.Config.Section("usage", &cfg); err != nil {
				d.Notice("Invalid usage configuration; using the default filters")
			}
			t := newTab(get(d), cfg)
			return []module.Module{&t}
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return checks(get(d)) },
	}
}
