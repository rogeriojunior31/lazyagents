// Package agents é a aba de visão geral dos agentes detectados: lista e
// detalhe com versão, skills ativas, sessões, diretórios lidos e o que o
// lazyagents gerencia em cada um. Não tem service nem CLI — os dados chegam
// pelos eventos do root; os adapters só respondem às type assertions.
package agents

import (
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Feature registra a aba.
func Feature() feature.Feature {
	return feature.Feature{
		Name: "agents",
		Last: true, // só informa: vai para o fim, como Uso
		Tabs: func(d *feature.Deps) []module.Module {
			t := newTab(d.Paths.Home, d.Adapters)
			return []module.Module{&t}
		},
	}
}
