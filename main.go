// lazyagents — TUI para gerenciar skills e sessões de agentes de coding AI.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/session"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "mostra a versão e sai")
	flag.Parse()

	if *showVersion {
		fmt.Printf("lazyagents %s\n", version)
		return
	}

	paths, err := skill.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
	adapters := agent.All(paths.Home)
	// migração única do layout legado (~/.lazyskills) para o padrão XDG.
	if _, err := skill.EnsureMigrated(paths, adapters); err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents: migração:", err)
	}
	// re-lê honrando o config.json já migrado (ex.: libraryDir custom).
	paths, err = skill.LoadPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
	skillSvc := skill.New(paths)
	sessionSvc := session.New(adapters, paths.BackupsDir())

	// subcomando presente → modo headless
	if flag.NArg() > 0 {
		agents := agent.DetectAll(adapters)
		os.Exit(cli.Run(flag.Args(), os.Stdout, os.Stderr, skillSvc, agents, sessionSvc))
	}

	// sem subcomando → TUI
	if _, err := tea.NewProgram(tui.New(adapters, skillSvc, sessionSvc, version)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
}
