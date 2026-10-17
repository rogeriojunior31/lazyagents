package recovery

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

func commands(svc *Service) []cli.Command {
	return []cli.Command{{Name: "config-backups",
		Usage:   "config-backups list|preview <id>|restore <id> [--query text] [--review token] [--json]",
		Summary: "inspect and recover agent configuration snapshots",
		Help: `subcommands:
  list                 list config snapshots (default), including unknown legacy origins
  preview <id>         verify origin/integrity and show a redacted before/after preview
  restore <id>         restore this file using the --review token from its preview

options:
  --query text         list: match agent, destination/file, operation and local date
  --json               list/preview: JSON output; config values are never included
  --review token       restore: required token from the exact reviewed preview

Use an exact ID from list. New snapshots carry adapter-provided provenance;
legacy snapshots without it are unknown and cannot be restored automatically.
The preview lists changed fields/lines with all values hidden. Restore replaces
the whole file, including unrelated edits listed in the preview. Current files
receive a safety backup; changed source, destination or permissions require a
new preview. Existing stricter permissions stay restricted.
Restore is per file: profiles, provider ownership records and other files are
not rolled back. Pi's two files need separate reviews. Codex hook trust/feature
approvals remain under native control and can prevent a config.toml restore.
Skill archives use skills backups/restore instead.`,
		Run: func(c cli.Context, args []string) int { return cmdConfigBackups(c, args, svc) }}}
}

func cmdConfigBackups(c cli.Context, args []string, svc *Service) int {
	sub := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	fs := cli.Flags("config-backups "+sub, c.Err)
	switch sub {
	case "list":
		query := fs.String("query", "", "agent, file, operation or local date")
		jsonOut := fs.Bool("json", false, "JSON output")
		if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
			fmt.Fprintln(c.Err, "usage: lazyagents config-backups list [--query text] [--json]")
			return 1
		}
		entries, err := svc.List()
		if err != nil {
			return configError(c, err)
		}
		entries = Filter(entries, *query)
		if *jsonOut {
			return writeJSON(c, entries)
		}
		tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tAGENT\tOPERATION\tDATE\tDESTINATION / STATUS")
		for _, entry := range entries {
			destination := entry.Destination
			if !entry.Restorable {
				destination = entry.Reason
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", entry.ID, entry.Agent, entry.Operation, entry.Time.Local().Format("2006-01-02 15:04:05"), destination)
		}
		if err := tw.Flush(); err != nil {
			return configError(c, err)
		}
		return 0
	case "preview", "restore":
		if len(args) == 0 {
			return configError(c, fmt.Errorf("usage: lazyagents config-backups %s <id> [options]", sub))
		}
		id := args[0]
		if sub == "preview" {
			jsonOut := fs.Bool("json", false, "JSON output")
			if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
				fmt.Fprintln(c.Err, "usage: lazyagents config-backups preview <id> [--json]")
				return 1
			}
			p, err := svc.Preview(id)
			if err != nil {
				return configError(c, err)
			}
			if *jsonOut {
				return writeJSON(c, p)
			}
			fmt.Fprintln(c.Out, previewText(p))
			fmt.Fprintf(c.Out, "\nTo replace this whole file after reviewing the differences:\nlazyagents config-backups restore %s --review %s\n", id, p.Review)
			return 0
		}
		review := fs.String("review", "", "token from the reviewed preview")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			fmt.Fprintln(c.Err, "usage: lazyagents config-backups restore <id> --review token")
			return 1
		}
		if err := svc.Restore(id, *review); err != nil {
			return configError(c, err)
		}
		fmt.Fprintln(c.Out, "config snapshot restored; other files and provider ownership records are unchanged")
		return 0
	}
	return configError(c, fmt.Errorf("unknown config-backups subcommand %q (list, preview, restore)", sub))
}

func configError(c cli.Context, err error) int {
	fmt.Fprintln(c.Err, "lazyagents:", err)
	return 1
}

func writeJSON(c cli.Context, value any) int {
	encoder := json.NewEncoder(c.Out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return configError(c, err)
	}
	return 0
}

func previewText(p Preview) string {
	lines := []string{
		"Backup: " + p.ID,
		"Agent: " + p.Agent + " | Operation: " + p.Operation,
		"Destination: " + p.Destination,
		fmt.Sprintf("Current file exists: %t", p.CurrentExists),
		"Permissions: " + p.CurrentMode + " -> " + p.RestoredMode,
		"Current -> backup: all values, commands and comments are hidden.",
		"This replaces the whole file, including unrelated changes below.",
		"Other files, profiles and provider ownership records are unchanged.",
		"",
	}
	if len(p.Changes) == 0 {
		lines = append(lines, "No field/line changes.")
	}
	return strings.Join(append(lines, p.Changes...), "\n")
}
