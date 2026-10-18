package recovery

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

func Feature() feature.Feature {
	var svc *Service
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Adapters, d.Paths.BackupsDir())
		}
		return svc
	}
	return feature.Feature{
		Name:     "recovery",
		Tabs:     func(d *feature.Deps) []module.Module { return []module.Module{newTab(get(d))} },
		Commands: func(d *feature.Deps) []cli.Command { return append(commands(get(d)), historyCommand(get(d))) },
	}
}
