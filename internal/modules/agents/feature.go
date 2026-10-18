// Package agents is the overview tab of the detected agents. It has no service
// or CLI: data arrives through root events and adapter type assertions.
package agents

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

func Feature(doctor cli.DoctorRunner) feature.Feature {
	var tab *Tab
	return feature.Feature{
		Name: "agents",
		Close: func() {
			if tab != nil {
				tab.shutdownDoctor()
			}
		},
		Last: true, // read-only, so it goes last like Usage
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(d.Paths.Home, d.Adapters)
			t.doctor.run = doctor
			t.detect = func() agent.Detection { return d.DetectAgents(true) }
			tab = &t
			return []module.Module{&t}
		},
	}
}
