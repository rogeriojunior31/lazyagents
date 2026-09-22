// Package agents é a aba de visão geral dos agentes detectados: um card por
// agente instalado, com versão, skills ativas, sessões e diretórios lidos.
// Não tem service nem CLI — os dados chegam pelos eventos do root.
package agents

import (
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registra a aba.
func Feature() feature.Feature {
	return feature.Feature{
		Name: "agents",
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab()
			return []module.Module{&t}
		},
	}
}
