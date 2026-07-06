// lazyskills — TUI para gerenciar skills e sessões de agentes de coding AI.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"lazyskills/internal/agent"
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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyskills:", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := skill.DefaultPaths()
	if err != nil {
		return err
	}
	adapters := agent.All(paths.Home)
	skillSvc := skill.New(paths)
	sessionSvc := session.New(adapters)

	_, err = tea.NewProgram(tui.New(adapters, skillSvc, sessionSvc)).Run()
	return err
}
