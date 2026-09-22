package app

import (
	"os"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/views"
)

// Feature é um módulo do lazyagents: uma aba, subcomandos de CLI e seções do
// doctor. Qualquer campo pode ser nil.
type Feature struct {
	Name      string
	NewModule func(d *Deps) module.Module
	Commands  func(d *Deps) []cli.Command
	Checks    func(d *Deps) []cli.Check
}

// Features é O registro. Ordem = ordem das abas e do help da CLI.
var Features = []Feature{
	{
		Name: "skills",
		NewModule: func(d *Deps) module.Module {
			m := views.NewSkills(d.Skills)
			return &m
		},
		Commands: func(d *Deps) []cli.Command { return cli.SkillCommands(d.Skills) },
		Checks:   func(d *Deps) []cli.Check { return cli.SkillChecks(d.Skills) },
	},
	{
		Name: "sessions",
		NewModule: func(d *Deps) module.Module {
			m := views.NewSessions(d.Sessions, d.Paths.Home)
			return &m
		},
		Commands: func(d *Deps) []cli.Command { return cli.SessionCommands(d.Sessions) },
	},
	{
		Name: "agents",
		NewModule: func(d *Deps) module.Module {
			m := views.NewAgents()
			return &m
		},
	},
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

// Commands junta os subcomandos de todas as features + o doctor agregado.
func (d *Deps) Commands() []cli.Command {
	var cmds []cli.Command
	var checks []cli.Check
	for _, f := range Features {
		if f.Commands != nil {
			cmds = append(cmds, f.Commands(d)...)
		}
		if f.Checks != nil {
			checks = append(checks, f.Checks(d)...)
		}
	}
	return append(cmds, cli.DoctorCommand(checks))
}

// RunCLI executa um subcomando headless e devolve o exit code.
func (d *Deps) RunCLI(args []string) int {
	c := cli.Context{Out: os.Stdout, Err: os.Stderr, Paths: d.Paths, Agents: d.Agents}
	return cli.Run(args, c, d.Commands())
}
