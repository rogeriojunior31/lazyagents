package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	usagesvc "github.com/rogeriojunior31/lazyagents/internal/usage"
)

// UsageCommands são os subcomandos da feature usage.
func UsageCommands(svc *usagesvc.Service) []Command {
	return []Command{
		{Name: "usage", Usage: "usage [--json] [--agent id] [--refresh]", Run: func(c Context, a []string) int {
			return cmdUsage(a, c, svc)
		}},
	}
}

func cmdUsage(args []string, c Context, svc *usagesvc.Service) int {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	jsonOut := fs.Bool("json", false, "saída JSON")
	agentID := fs.String("agent", "", "só este agente")
	refresh := fs.Bool("refresh", false, "ignora o cache e consulta de novo")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	statuses := svc.Status(ctx, *refresh)
	var out []usagesvc.Status
	for _, st := range statuses {
		if *agentID == "" || st.AgentID == *agentID {
			out = append(out, st)
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return exitFor(out)
	}
	if len(out) == 0 {
		fmt.Fprintln(c.Err, "lazyagents: nenhum agente com informação de uso")
		return 1
	}
	tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENTE\tCONTA\tPLANO\tJANELA\tUSADO\tRESET")
	for _, st := range out {
		plan := st.Limits.Plan
		if plan == "" {
			plan = "-"
		}
		if len(st.Limits.Windows) == 0 {
			detail := "sem limites"
			if st.Err != "" {
				detail = st.Err
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t-\t-\n", st.AgentID, st.AuthLabel, plan, detail)
			continue
		}
		for _, w := range st.Limits.Windows {
			reset := "-"
			if !w.ResetsAt.IsZero() {
				reset = w.ResetsAt.Local().Format("02/01 15:04")
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.1f%%\t%s\n", st.AgentID, st.AuthLabel, plan, w.Label, w.UsedPercent, reset)
		}
	}
	_ = tw.Flush()
	return exitFor(out)
}

// exitFor devolve 1 quando nenhum agente conseguiu informar limites.
func exitFor(sts []usagesvc.Status) int {
	for _, st := range sts {
		if len(st.Limits.Windows) > 0 {
			return 0
		}
	}
	return 1
}

// UsageChecks reporta no doctor como cada agente está autenticado e se os
// limites foram obtidos (usa o cache; não força rede).
func UsageChecks(svc *usagesvc.Service) []Check {
	return []Check{{Title: "uso", Run: func(c Context, out io.Writer) []string {
		var problems []string
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		for _, st := range svc.Status(ctx, false) {
			switch {
			case st.Err != "":
				fmt.Fprintf(out, "  ✗ %-16s %s\n", st.AgentID, st.Err)
				problems = append(problems, "uso de "+st.AgentID+": "+st.Err)
			default:
				fmt.Fprintf(out, "  ✓ %-16s %s · %d janela(s)\n", st.AgentID, st.AuthLabel, len(st.Limits.Windows))
			}
		}
		return problems
	}}}
}
