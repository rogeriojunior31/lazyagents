package app

import (
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/modules/agents"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins"
	"github.com/rogeriojunior31/lazyagents/internal/modules/providers"
	"github.com/rogeriojunior31/lazyagents/internal/modules/sessions"
	"github.com/rogeriojunior31/lazyagents/internal/modules/skills"
	"github.com/rogeriojunior31/lazyagents/internal/modules/usage"
)

// features é O registro: uma linha por módulo, e a ordem é a ordem das abas
// e do help da CLI.
//
// O módulo de plugins vem por último porque descobre ids em runtime e precisa
// dos nomes embutidos já reservados; usage se declara Last e por isso a aba
// dele sai no fim da barra, depois até das abas de plugin.
func features() []feature.Feature {
	return []feature.Feature{
		skills.Feature(),
		sessions.Feature(),
		agents.Feature(),
		providers.Feature(),
		hooks.Feature(),
		usage.Feature(),
		plugins.Feature(),
	}
}
