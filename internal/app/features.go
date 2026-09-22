package app

import (
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/views"
)

// Feature é um módulo do lazyagents: uma aba e/ou comandos de CLI.
type Feature struct {
	Name      string
	NewModule func(d *Deps) module.Module // nil = só CLI
}

// Features é O registro. Ordem = ordem das abas.
var Features = []Feature{
	{Name: "skills", NewModule: func(d *Deps) module.Module {
		m := views.NewSkills(d.Skills)
		return &m
	}},
	{Name: "sessions", NewModule: func(d *Deps) module.Module {
		m := views.NewSessions(d.Sessions, d.Paths.Home)
		return &m
	}},
	{Name: "agents", NewModule: func(d *Deps) module.Module {
		m := views.NewAgents()
		return &m
	}},
}

// Modules instancia as abas de todas as features, na ordem do registro.
func (d *Deps) Modules() []module.Module {
	var mods []module.Module
	for _, f := range Features {
		if f.NewModule != nil {
			mods = append(mods, f.NewModule(d))
		}
	}
	return mods
}
