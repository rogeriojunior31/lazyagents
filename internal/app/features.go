package app

import (
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/modules/agents"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins"
	"github.com/rogeriojunior31/lazyagents/internal/modules/providers"
	"github.com/rogeriojunior31/lazyagents/internal/modules/recovery"
	"github.com/rogeriojunior31/lazyagents/internal/modules/sessions"
	"github.com/rogeriojunior31/lazyagents/internal/modules/skills"
	"github.com/rogeriojunior31/lazyagents/internal/modules/usage"
)

// features is THE registry: one line per module, in tab and CLI help order.
// plugins comes last because it discovers ids at runtime and needs the
// built-in names already reserved; usage and agents set Last, so they end
// the tab bar, after plugin tabs too.
func features(doctor cli.DoctorRunner) []feature.Feature {
	return []feature.Feature{
		skills.Feature(),
		sessions.Feature(),
		providers.Feature(),
		hooks.Feature(),
		recovery.Feature(),
		usage.Feature(),
		agents.Feature(doctor),
		plugins.Feature(),
	}
}
