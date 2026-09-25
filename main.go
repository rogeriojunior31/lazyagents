// lazyagents is a TUI to manage the skills, sessions and settings of AI coding
// agents. Composition (paths, services, modules) lives in internal/app.
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
	showVersion := flag.Bool("version", false, "print the version and exit")
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

	// user themes before any Apply; a broken file becomes a notice
	for _, err := range theme.LoadUser(a.Deps.Paths.ThemesDir()) {
		a.Deps.Notice(err.Error())
	}

	// An invalid theme falls back to the default with a notice (CLI: before the
	// command; TUI: on stderr after the alt screen closes).
	if err := theme.Apply(a.Deps.Config.Theme); err != nil {
		_ = theme.Apply(theme.Default)
		a.Deps.Notice(fmt.Sprintf("%v in %s; using %q", err, a.Deps.Paths.ConfigPath(), theme.Default))
	}

	// subcommand → headless mode
	if flag.NArg() > 0 {
		code := a.RunCLI(flag.Args())
		a.Close()
		os.Exit(code)
	}

	// no subcommand → TUI
	defer func() {
		for _, n := range a.Deps.Notices() {
			fmt.Fprintln(os.Stderr, "lazyagents:", n)
		}
	}()
	mods, opts := a.Layout()
	_, err = tea.NewProgram(tui.New(mods, a.Deps.Adapters, version, opts)).Run()
	a.Close() // stop plugins before any exit
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazyagents:", err)
		os.Exit(1)
	}
}
