// Package app é a raiz de composição do lazyagents: monta paths e config
// (Load), instancia os módulos registrados e expõe TUI e CLI. Só main importa
// app.
//
// Adicionar um módulo novo = um pacote em internal/modules/<nome> com a sua
// Feature e UMA linha em features().
package app

import (
	"fmt"
	"os"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// App é o lazyagents montado: as dependências compartilhadas e os módulos
// registrados.
type App struct {
	Deps     *feature.Deps
	features []feature.Feature
}

// Load executa o boot: paths XDG, config.yaml, adapters e módulos.
func Load(version string) (*App, error) {
	paths, err := core.DefaultPaths()
	if err != nil {
		return nil, err
	}
	return LoadWith(paths, version)
}

// LoadWith é Load com paths injetados (testes).
func LoadWith(paths core.Paths, version string) (*App, error) {
	d := &feature.Deps{Version: version}
	if migrated, err := core.MigrateConfig(paths); err != nil {
		d.Notice(err.Error())
	} else if migrated {
		d.Notice("config.json migrado para " + paths.ConfigPath())
	}
	paths = paths.WithConfig() // honra overrides do config.yaml (ex.: libraryDir)
	cfg, err := core.ReadConfig(paths.ConfigPath())
	if err != nil {
		d.Notice(err.Error() + "; usando padrões") // config inválida nunca trava o boot
	}
	d.Paths, d.Config, d.Adapters = paths, cfg, agent.All(paths.Home)

	a := &App{Deps: d, features: features()}
	a.reserveNames()
	return a, nil
}

// reserveNames marca os nomes de abas e comandos embutidos. Quem cria abas em
// runtime (plugins) consulta isso para não colidir — por isso o módulo de
// plugins é o último do registro: quando ele é consultado, todo o resto já
// está reservado.
func (a *App) reserveNames() {
	a.Deps.Reserve("doctor", "help")
	for _, f := range a.features {
		a.Deps.Reserve(f.Name)
		if f.Commands == nil {
			continue
		}
		for _, c := range f.Commands(a.Deps) {
			a.Deps.Reserve(c.Name)
		}
	}
}

// Modules instancia as abas na ordem do registro; as features marcadas com
// Last vão para o fim, depois até das abas criadas em runtime.
func (a *App) Modules() []module.Module {
	var mods, last []module.Module
	for _, f := range a.features {
		if f.Tabs == nil {
			continue
		}
		if f.Last {
			last = append(last, f.Tabs(a.Deps)...)
			continue
		}
		mods = append(mods, f.Tabs(a.Deps)...)
	}
	return append(mods, last...)
}

// Commands junta os subcomandos de todos os módulos + o doctor agregado.
func (a *App) Commands() []cli.Command {
	var cmds []cli.Command
	var checks []cli.Check
	for _, f := range a.features {
		if f.Commands != nil {
			cmds = append(cmds, f.Commands(a.Deps)...)
		}
		if f.Checks != nil {
			checks = append(checks, f.Checks(a.Deps)...)
		}
	}
	return append(cmds, cli.DoctorCommand(checks))
}

// RunCLI executa um subcomando headless e devolve o exit code.
func (a *App) RunCLI(args []string) int {
	for _, n := range a.Deps.Notices() {
		fmt.Fprintln(os.Stderr, "lazyagents:", n)
	}
	ids := make([]string, 0, len(a.Deps.Adapters))
	for _, ad := range a.Deps.Adapters {
		ids = append(ids, ad.ID())
	}
	c := cli.Context{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Paths: a.Deps.Paths, Agents: a.Deps.Agents, AgentIDs: ids}
	return cli.Run(args, c, a.Commands())
}

// Close encerra os recursos dos módulos (processos de plugin); chamar ao sair.
func (a *App) Close() {
	for _, f := range a.features {
		if f.Close != nil {
			f.Close()
		}
	}
}
