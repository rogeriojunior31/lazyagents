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

	a, err := app.Load(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}

	// subcomando presente → modo headless
	if flag.NArg() > 0 {
		_ = theme.Apply(a.Deps.Config.Theme) // cores da saída; tema inválido fica no padrão
		code := a.RunCLI(flag.Args())
		a.Close()
		os.Exit(code)
	}

	// sem subcomando → TUI. Tema inválido não impede a abertura: cai no padrão
	// e os avisos (tema, migração) saem no stderr depois que a tela alternativa fecha.
	if err := theme.Apply(a.Deps.Config.Theme); err != nil {
		_ = theme.Apply(theme.Default)
		a.Deps.Notice(fmt.Sprintf("%v em %s; usando %q", err, a.Deps.Paths.ConfigPath(), theme.Default))
	}
	defer func() {
		for _, n := range a.Deps.Notices() {
			fmt.Fprintln(os.Stderr, "lazyagents:", n)
		}
	}()
	mods, opts := a.Layout()
	_, err = tea.NewProgram(tui.New(mods, a.Deps.Adapters, version, opts)).Run()
	a.Close() // encerra os plugins antes de qualquer saída
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
}
