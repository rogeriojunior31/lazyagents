package plugins

import (
	"fmt"
	"io"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

// commands exposes each plugin as `lazyagents <id> [args…]` (pass-through).
func commands(svc *Service, pls []Plugin) []cli.Command {
	cmds := make([]cli.Command, 0, len(pls))
	for _, pl := range pls {
		cmds = append(cmds, cli.Command{
			Name:    pl.ID,
			Usage:   pl.ID + " [args…]",
			Summary: "external plugin",
			Help:    fmt.Sprintf("Runs the %s plugin (%s) with the given arguments: stdin, stdout,\nstderr and the exit code pass through unchanged.", pl.ID, pl.Path),
			Run:     func(c cli.Context, args []string) int { return svc.Run(pl, args, c.In, c.Out, c.Err) },
		})
	}
	return cmds
}

// checks is the doctor's plugins section: discovery warnings, each plugin's
// handshake and, when the manifest asks for it, its `<bin> doctor`.
func checks(svc *Service, pls []Plugin, initFor func(Plugin) Msg, warnings []string) []cli.Check {
	return []cli.Check{{Title: "plugins", Run: func(c cli.Context, out io.Writer) []string {
		problems := append([]string(nil), warnings...)
		for _, w := range warnings {
			fmt.Fprintf(out, "  ✗ %s\n", w)
		}
		if len(pls) == 0 && len(warnings) == 0 {
			fmt.Fprintf(out, "  no plugins in %s\n", svc.Dir)
		}
		for _, pl := range pls {
			p, err := svc.Start(pl, initFor(pl))
			if err != nil {
				fmt.Fprintf(out, "  ✗ %s: %v\n", pl.ID, err)
				problems = append(problems, err.Error())
				continue
			}
			fmt.Fprintf(out, "  ✓ %-16s %s\n", pl.ID, p.Manifest.Title)
			doctor := p.Manifest.Doctor
			_ = p.Close()
			if !doctor {
				continue
			}
			switch ok, err := svc.RunDoctor(pl, out); {
			case err != nil:
				fmt.Fprintf(out, "  ✗ %v\n", err)
				problems = append(problems, err.Error())
			case !ok:
				problems = append(problems, fmt.Sprintf("plugin %s: doctor reported a problem", pl.ID))
			}
		}
		return problems
	}}}
}
