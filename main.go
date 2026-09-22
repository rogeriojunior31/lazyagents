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
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
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

	// sem subcomando → TUI. Tema inválido não impede a abertura: cai no padrão
	// e os avisos (tema, migração) saem no stderr depois que a tela alternativa fecha.
	if err := theme.Apply(deps.Config.Theme); err != nil {
		_ = theme.Apply(theme.Default)
		deps.Notices = append(deps.Notices, fmt.Sprintf("%v em %s; usando %q", err, deps.Paths.ConfigPath(), theme.Default))
	}
	defer func() {
		for _, n := range deps.Notices {
			fmt.Fprintln(os.Stderr, "lazyagents:", n)
		}
	}()
	_, err = tea.NewProgram(tui.New(deps.Modules(), deps.Adapters, version)).Run()
	deps.Close() // encerra os plugins antes de qualquer saída
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
}
