// lazyagents — TUI para gerenciar skills, sessões e configurações dos agentes
// de coding AI. A composição (paths, services, módulos) vive em internal/app.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/app"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
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

	deps, warning, err := app.Load(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
	if warning != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", warning)
	}

	// subcomando presente → modo headless
	if flag.NArg() > 0 {
		os.Exit(cli.Run(flag.Args(), os.Stdout, os.Stderr, deps.Skills, deps.Agents(), deps.Sessions))
	}

	// sem subcomando → TUI
	if _, err := tea.NewProgram(tui.New(deps.Modules(), deps.Adapters, version)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
}
