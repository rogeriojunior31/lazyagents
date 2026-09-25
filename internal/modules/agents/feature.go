// Package agents is the overview tab of the detected agents. It has no service
// or CLI: data arrives through root events and adapter type assertions.
package agents

import (
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

func Feature() feature.Feature {
	return feature.Feature{
		Name: "agents",
		Last: true, // read-only, so it goes last like Usage
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(d.Paths.Home, d.Adapters)
			return []module.Module{&t}
		},
	}
}
