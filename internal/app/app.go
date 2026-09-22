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
	// Config é o config.yaml lido no boot (zero se ausente/inválido). O tema é
	// validado por quem aplica (main); módulos leem sua seção com Config.Section.
	Config core.Config
	// Notices são avisos de boot (migração, config inválida) para o stderr:
	// a TUI imprime ao fechar a tela alternativa, a CLI imprime na hora.
	Notices []string

	detectOnce sync.Once
	agents     []agent.Agent
}

// Agents devolve a detecção dos agentes, memoizada (Detect roda --version de
// cada CLI; só paga quando alguém pede).
func (d *Deps) Agents() []agent.Agent {
	d.detectOnce.Do(func() { d.agents = agent.DetectAll(d.Adapters) })
	return d.agents
}

// Load executa o boot: paths XDG, config.yaml, adapters e services.
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
	var notices []string
	if migrated, err := core.MigrateConfig(paths); err != nil {
		notices = append(notices, err.Error())
	} else if migrated {
		notices = append(notices, "config.json migrado para "+paths.ConfigPath())
	}
	paths = paths.WithConfig() // honra overrides do config.yaml (ex.: libraryDir)
	cfg, err := core.ReadConfig(paths.ConfigPath())
	if err != nil {
		notices = append(notices, err.Error()+"; usando padrões") // config inválida nunca trava o boot
	}
	return &Deps{
		Config:   cfg,
		Notices:  notices,
		Paths:    paths,
		Adapters: adapters,
		Skills:   skill.New(paths),
		Sessions: session.New(adapters, paths),
		Version:  version,
	}, nil
}
