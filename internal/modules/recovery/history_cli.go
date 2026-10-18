package recovery

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func historyCommand(svc *Service) cli.Command {
	return cli.Command{Name: "history", Usage: "history list|inspect <id>|clear [--query text] [--json] [--yes]", Summary: "inspect recent local write attempts without replaying them", Help: `subcommands:
  list                 list recent attempts, newest first (default)
  inspect <id>         inspect target outcomes and verify related config backups
  clear --yes          clear history only; keep all backups and current settings

options:
  --query text         list: match module, action, resource, target, outcome or local date
  --json               list/inspect: metadata only; no errors, bodies or commands
  --yes                clear: confirm removing local history

Keep at most 200 attempts, 1000 targets and 32 related backups per attempt,
within 32 MiB. The owner-only history file is separate from backup retention.
Concurrent history updates are serialized; persistence failures are reported.
History is not an executable retry plan. Inspect current state and review the
operation again in its module. A config backup requires a fresh config-backups
preview and its review token; unavailable/rotated backups cannot be restored.
Related snapshots match the attempt time, action and target; overlapping
attempts may reference the same snapshot. No operation is replayed here.`, Run: func(c cli.Context, args []string) int { return cmdHistory(c, args, svc) }}
}

func cmdHistory(c cli.Context, args []string, svc *Service) int {
	sub := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	fs := cli.Flags("history "+sub, c.Err)
	switch sub {
	case "list":
		query := fs.String("query", "", "match metadata")
		jsonOut := fs.Bool("json", false, "JSON metadata")
		if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
			return configError(c, fmt.Errorf("usage: lazyagents history list [--query text] [--json]"))
		}
		entries, err := svc.historyEntries()
		if err != nil {
			return configError(c, err)
		}
		matches := []core.HistoryEntry{}
		for _, e := range entries {
			text := strings.ToLower(e.ID + " " + e.Module + " " + e.Action + " " + e.Subject + " " + e.Time.Local().Format("2006-01-02 15:04:05"))
			for _, t := range e.Targets {
				text += " " + strings.ToLower(t.ID+" "+string(t.Outcome))
			}
			match := true
			for _, term := range strings.Fields(strings.ToLower(*query)) {
				match = match && strings.Contains(text, term)
			}
			if match {
				matches = append(matches, e)
			}
		}
		if *jsonOut {
			return writeJSON(c, matches)
		}
		tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tDATE\tMODULE / ACTION\tOUTCOME")
		for _, e := range matches {
			fmt.Fprintf(tw, "%s\t%s\t%s / %s\t%s\n", e.ID, e.Time.Local().Format("2006-01-02 15:04:05"), e.Module, e.Action, historySummary(e))
		}
		if err := tw.Flush(); err != nil {
			return configError(c, err)
		}
		return 0
	case "inspect":
		if len(args) == 0 {
			return configError(c, fmt.Errorf("usage: lazyagents history inspect <id> [--json]"))
		}
		id := args[0]
		jsonOut := fs.Bool("json", false, "JSON metadata")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			return configError(c, fmt.Errorf("usage: lazyagents history inspect <id> [--json]"))
		}
		p, err := svc.inspectHistory(id)
		if err != nil {
			return configError(c, err)
		}
		if *jsonOut {
			return writeJSON(c, p)
		}
		fmt.Fprintln(c.Out, historyText(p))
		for _, b := range p.Backups {
			if b.Available {
				fmt.Fprintf(c.Out, "\nReview current file against this backup:\nlazyagents config-backups preview %s\n", b.ID)
			}
		}
		return 0
	case "clear":
		yes := fs.Bool("yes", false, "confirm clearing history")
		if err := fs.Parse(args); err != nil || fs.NArg() != 0 || !*yes {
			return configError(c, fmt.Errorf("usage: lazyagents history clear --yes (backups are kept)"))
		}
		if err := svc.clearHistory(); err != nil {
			return configError(c, err)
		}
		fmt.Fprintln(c.Out, "Operation history cleared; backups and current settings are kept")
		return 0
	}
	return configError(c, fmt.Errorf("unknown history subcommand (list, inspect, clear)"))
}
