package plugins

import (
	"fmt"
	"io"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

// commands expõe cada plugin como `lazyagents <id> [args…]` (pass-through).
func commands(svc *Service, pls []Plugin) []cli.Command {
	cmds := make([]cli.Command, 0, len(pls))
	for _, pl := range pls {
		cmds = append(cmds, cli.Command{
			Name:  pl.ID,
			Usage: pl.ID + " [args…]  (plugin)",
			Run:   func(c cli.Context, args []string) int { return svc.Run(pl, args, c.In, c.Out, c.Err) },
		})
	}
	return cmds
}

// checks é a seção "plugins" do doctor: avisos da descoberta, handshake de
// cada plugin e, quando o manifesto pede, o `<bin> doctor` dele.
func checks(svc *Service, pls []Plugin, initFor func(Plugin) Msg, warnings []string) []cli.Check {
	return []cli.Check{{Title: "plugins", Run: func(c cli.Context, out io.Writer) []string {
		problems := append([]string(nil), warnings...)
		for _, w := range warnings {
			fmt.Fprintf(out, "  ✗ %s\n", w)
		}
		if len(pls) == 0 && len(warnings) == 0 {
			fmt.Fprintf(out, "  nenhum plugin em %s\n", svc.Dir)
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
			if doctor && svc.Run(pl, []string{"doctor"}, nil, out, out) != 0 {
				problems = append(problems, "plugin "+pl.ID+": doctor reportou problema")
			}
		}
		return problems
	}}}
}
