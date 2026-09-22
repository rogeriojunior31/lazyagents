// Package app é a raiz de composição do lazyagents: monta paths, adapters e
// services (Load) e registra as features (Features). Só main importa app.
//
// Adicionar um módulo novo = service em internal/<dominio>, aba em
// internal/tui/..., comandos em internal/cli e UMA entrada em Features.
package app

import (
	"sync"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/session"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
)

// Deps são as dependências compartilhadas entregues às features. Cada módulo
// recebe daqui só o que precisa.
type Deps struct {
	Paths    core.Paths
	Adapters []agent.Adapter
	Skills   *skill.Service
	Sessions *session.Service
	Version  string
	// Theme é o tema da TUI em config.json ("" = padrão). Validado por quem aplica.
	Theme string

	detectOnce sync.Once
	agents     []agent.Agent
}

// Agents devolve a detecção dos agentes, memoizada (Detect roda --version de
// cada CLI; só paga quando alguém pede).
func (d *Deps) Agents() []agent.Agent {
	d.detectOnce.Do(func() { d.agents = agent.DetectAll(d.Adapters) })
	return d.agents
}

// Load executa o boot: paths XDG, config.json, adapters e services.
func Load(version string) (*Deps, error) {
	paths, err := core.DefaultPaths()
	if err != nil {
		return nil, err
	}
	return LoadWith(paths, version)
}

// LoadWith é Load com paths injetados (testes).
func LoadWith(paths core.Paths, version string) (*Deps, error) {
	adapters := agent.All(paths.Home)
	paths = paths.WithConfig()                          // honra overrides do config.json (ex.: libraryDir)
	_, cfg, _ := core.ReadConfigRaw(paths.ConfigPath()) // config inválida nunca trava o boot
	return &Deps{
		Theme:    cfg.Theme,
		Paths:    paths,
		Adapters: adapters,
		Skills:   skill.New(paths),
		Sessions: session.New(adapters, paths),
		Version:  version,
	}, nil
}
