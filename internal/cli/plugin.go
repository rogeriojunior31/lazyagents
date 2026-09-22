package cli

import (
	"fmt"
	"io"

	"github.com/rogeriojunior31/lazyagents/internal/plugin"
)

// PluginCommands expõe cada plugin como `lazyagents <id> [args…]` (pass-through).
func PluginCommands(svc *plugin.Service, pls []plugin.Plugin) []Command {
	cmds := make([]Command, 0, len(pls))
	for _, pl := range pls {
		cmds = append(cmds, Command{
			Name:  pl.ID,
			Usage: pl.ID + " [args…]  (plugin)",
			Run:   func(c Context, args []string) int { return svc.Run(pl, args, c.In, c.Out, c.Err) },
		})
	}
	return cmds
}

// PluginChecks é a seção "plugins" do doctor: avisos da descoberta, handshake
// de cada plugin e, quando o manifesto pede, o `<bin> doctor` dele.
func PluginChecks(svc *plugin.Service, pls []plugin.Plugin, initFor func(plugin.Plugin) plugin.Msg, warnings []string) []Check {
	return []Check{{Title: "plugins", Run: func(c Context, out io.Writer) []string {
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
