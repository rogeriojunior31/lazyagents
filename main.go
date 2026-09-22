// lazyagents — TUI para gerenciar skills, sessões e configurações dos agentes
// de coding AI. A composição (paths, services, módulos) vive em internal/app.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/app"
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

	deps, err := app.Load(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}

	// subcomando presente → modo headless
	if flag.NArg() > 0 {
		os.Exit(deps.RunCLI(flag.Args()))
	}

	// sem subcomando → TUI
	if _, err := tea.NewProgram(tui.New(deps.Modules(), deps.Adapters, version)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
}
