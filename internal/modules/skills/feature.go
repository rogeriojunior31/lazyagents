package skills

import (
	"os"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registers the skills module: tab, library commands and the doctor
// section that validates each SKILL.md.
func Feature() feature.Feature {
	var svc *Service
	var tab *Tab
	get := func(d *feature.Deps) *Service {
		if svc == nil {
			svc = New(d.Paths)
		}
		return svc
	}
	return feature.Feature{
		Name: "skills",
		Close: func() {
			if tab != nil {
				tab.sourceOp.close()
				if tab.review != nil {
					tab.review.Close()
				}
				if tab.picker.cleanup != "" {
					_ = os.RemoveAll(tab.picker.cleanup)
				}
			}
		},
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(get(d))
			tab = &t
			return []module.Module{&t}
		},
		Commands: func(d *feature.Deps) []cli.Command { return commands(get(d)) },
		Checks:   func(d *feature.Deps) []cli.Check { return checks(get(d)) },
	}
}
