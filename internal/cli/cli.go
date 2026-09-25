// Package cli is the headless framework: dispatch, generated help and flags.
// Features contribute Commands and doctor Checks; the registry is in internal/app.
package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Context is what every command receives.
type Context struct {
	In       io.Reader // stdin for plugin pass-through; nil = no input
	Out, Err io.Writer
	Paths    core.Paths
	Agents   func() []agent.Agent // memoized detection (runs each CLI's --version)
	AgentIDs []string             // every adapter id, no detection; nil = no validation
}

// KnownAgent reports whether id is a supported agent, listing the valid ones
// on stderr if not. An empty id (no filter) is always accepted.
func (c Context) KnownAgent(id string) bool {
	if id == "" || c.AgentIDs == nil || slices.Contains(c.AgentIDs, id) {
		return true
	}
	fmt.Fprintf(c.Err, "lazyagents: unknown agent %q (valid: %s)\n", id, strings.Join(c.AgentIDs, ", "))
	return false
}

// Command is a subcommand; Run parses its own flags (Flags) and returns the
// exit code.
type Command struct {
	Name    string
	Usage   string // one line, without the "lazyagents " prefix
	Summary string // one short sentence for the help list
	Help    string // optional detail for `help <command>`
	Run     func(c Context, args []string) int
}

// Run dispatches args[0] to the command with that name.
func Run(args []string, c Context, cmds []Command) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		if len(args) > 1 {
			return help(c, cmds, args[1])
		}
		fmt.Fprint(c.Err, usage(cmds))
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	if cmd, ok := find(cmds, args[0]); ok {
		return cmd.Run(c, args[1:])
	}
	fmt.Fprintf(c.Err, "lazyagents: unknown command %q (see lazyagents help)\n", args[0])
	return 1
}

func find(cmds []Command, name string) (Command, bool) {
	for _, cmd := range cmds {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return Command{}, false
}

// help prints a command's detail.
func help(c Context, cmds []Command, name string) int {
	cmd, ok := find(cmds, name)
	if !ok {
		fmt.Fprintf(c.Err, "lazyagents: unknown command %q (see lazyagents help)\n", name)
		return 1
	}
	fmt.Fprintf(c.Out, "usage: lazyagents %s\n", cmd.Usage)
	if cmd.Summary != "" {
		fmt.Fprintf(c.Out, "\n%s\n", cmd.Summary)
	}
	if cmd.Help != "" {
		fmt.Fprintf(c.Out, "\n%s\n", strings.TrimRight(cmd.Help, "\n"))
	}
	return 0
}

func usage(cmds []Command) string {
	width := 0
	for _, cmd := range cmds {
		width = max(width, len(cmd.Name))
	}
	var b strings.Builder
	b.WriteString("usage: lazyagents <command> [options]\n")
	b.WriteString("       lazyagents help <command>   details and options of a command\n")
	b.WriteString("       lazyagents                  open the TUI\n\ncommands:\n")
	for _, cmd := range cmds {
		desc := cmd.Summary
		if desc == "" {
			desc = strings.TrimSpace(strings.TrimPrefix(cmd.Usage, cmd.Name))
		}
		fmt.Fprintf(&b, "  %-*s  %s\n", width, cmd.Name, desc)
	}
	return b.String()
}

// Flags builds a command FlagSet that reports errors on errOut without exiting.
func Flags(name string, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintf(errOut, "usage: lazyagents %s [options]\n", name)
		n := 0
		fs.VisitAll(func(*flag.Flag) { n++ })
		if n > 0 {
			fmt.Fprintln(errOut, "\noptions:")
			fs.PrintDefaults()
		}
	}
	return fs
}
