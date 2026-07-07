// lazyskills — TUI para gerenciar skills e sessões de agentes de coding AI.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/cli"
	"lazyskills/internal/session"
	"lazyskills/internal/skill"
	"lazyskills/internal/tui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "mostra a versão e sai")
	flag.Parse()

	if *showVersion {
		fmt.Printf("lazyskills %s\n", version)
		return
	}

	paths, err := skill.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyskills:", err)
		os.Exit(1)
	}
	adapters := agent.All(paths.Home)
	skillSvc := skill.New(paths)
	sessionSvc := session.New(adapters)

	// subcomando presente → modo headless
	if flag.NArg() > 0 {
		agents := agent.DetectAll(adapters)
		os.Exit(cli.Run(flag.Args(), os.Stdout, os.Stderr, skillSvc, agents, sessionSvc))
	}

	// sem subcomando → TUI
	if _, err := tea.NewProgram(tui.New(adapters, skillSvc, sessionSvc, version)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyskills:", err)
		os.Exit(1)
	}
}
