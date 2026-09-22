package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/plugin"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/agents"
	pluginmod "github.com/rogeriojunior31/lazyagents/internal/tui/modules/plugin"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/sessions"
	"github.com/rogeriojunior31/lazyagents/internal/tui/modules/skills"
	usagemod "github.com/rogeriojunior31/lazyagents/internal/tui/modules/usage"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
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
			m := skills.NewSkills(d.Skills)
			return &m
		},
		Commands: func(d *Deps) []cli.Command { return cli.SkillCommands(d.Skills) },
		Checks:   func(d *Deps) []cli.Check { return cli.SkillChecks(d.Skills) },
	},
	{
		Name: "sessions",
		NewModule: func(d *Deps) module.Module {
			m := sessions.NewSessions(d.Sessions, d.Paths.Home)
			return &m
		},
		Commands: func(d *Deps) []cli.Command { return cli.SessionCommands(d.Sessions) },
	},
	{
		Name: "usage",
		NewModule: func(d *Deps) module.Module {
			m := usagemod.NewUsage(d.Usage)
			return &m
		},
		Commands: func(d *Deps) []cli.Command { return cli.UsageCommands(d.Usage) },
		Checks:   func(d *Deps) []cli.Check { return cli.UsageChecks(d.Usage) },
	},
	{
		Name: "agents",
		NewModule: func(d *Deps) module.Module {
			m := agents.NewAgents()
			return &m
		},
	},
}

// Modules instancia as abas de todas as features, na ordem do registro, e
// depois uma aba por plugin externo (handshake síncrono: a paleta precisa do
// manifesto antes do Init).
func (d *Deps) Modules() []module.Module {
	var mods []module.Module
	for _, f := range Features {
		if f.NewModule != nil {
			mods = append(mods, f.NewModule(d))
		}
	}
	for _, pl := range d.plugins {
		mods = append(mods, pluginmod.New(d.Plugins, pl, d.pluginInit(pl)))
	}
	return mods
}

// pluginInit monta a mensagem init de um plugin: paths, tema ativo e a seção
// <id>: do config.yaml como JSON.
func (d *Deps) pluginInit(pl plugin.Plugin) plugin.Msg {
	m := plugin.Msg{
		Home: d.Paths.Home, ConfigDir: d.Paths.ConfigDir, DataDir: d.Paths.DataDir, LibraryDir: d.Paths.LibraryDir(),
		Theme: &plugin.Theme{ID: theme.Current()},
	}
	for _, p := range theme.Options() {
		if p.ID == theme.Current() {
			m.Theme.Colors = p.Colors
		}
	}
	var section any
	if err := d.Config.Section(pl.ID, &section); err == nil && section != nil {
		m.Config, _ = json.Marshal(section)
	}
	return m
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
	cmds = append(cmds, cli.PluginCommands(d.Plugins, d.plugins)...)
	checks = append(checks, cli.PluginChecks(d.Plugins, d.plugins, d.pluginInit, d.pluginWarn)...)
	return append(cmds, cli.DoctorCommand(checks))
}

// RunCLI executa um subcomando headless e devolve o exit code.
func (d *Deps) RunCLI(args []string) int {
	for _, n := range d.Notices {
		fmt.Fprintln(os.Stderr, "lazyagents:", n)
	}
	c := cli.Context{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Paths: d.Paths, Agents: d.Agents}
	return cli.Run(args, c, d.Commands())
}
