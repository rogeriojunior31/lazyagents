package hooks

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

// commands são os subcomandos da CLI deste módulo.
func commands(svc *Service) []cli.Command {
	return []cli.Command{
		{Name: "hooks", Usage: "hooks list|enable <name>|disable <name>|add <name>|rm <name> [--agent id] [--json]",
			Summary: "list library hooks and install/uninstall them in agents",
			Run:     func(c cli.Context, a []string) int { return cmdHooks(a, c, svc) }},
	}
}

func cmdHooks(args []string, c cli.Context, svc *Service) int {
	sub := ""
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "list", "":
		return hooksList(args, c, svc)
	case "enable":
		return hooksToggle(args, c, svc, true)
	case "disable":
		return hooksToggle(args, c, svc, false)
	case "add":
		return hooksAdd(args, c, svc)
	case "rm":
		return hooksRemove(args, c, svc)
	default:
		fmt.Fprintf(c.Err, "lazyagents hooks: unknown subcommand %q (list, enable, disable, add, rm)\n", sub)
		return 1
	}
}

func hooksList(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("hooks list", c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	lib, problems := svc.Library()
	statuses := svc.Status()

	if *jsonOut {
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Hooks  []Hook   `json:"hooks"`
			Agents []Status `json:"agents"`
		}{lib, statuses})
		return 0
	}

	tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOOK\tEVENTS\tCOMMANDS\tINSTALLED IN")
	for _, h := range lib {
		var in []string
		for _, st := range statuses {
			if enabledIn(st, h.Name) {
				in = append(in, st.AgentID)
			} else if partialIn(st, h.Name) {
				in = append(in, st.AgentID+" (partial)")
			}
		}
		where := "-"
		if len(in) > 0 {
			where = strings.Join(in, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.Name, strings.Join(h.Events(), ","), h.Summary(), where)
	}
	if len(lib) == 0 {
		fmt.Fprintln(tw, "(empty library)\t\t\t")
	}
	fmt.Fprintln(tw, "\t\t\t\t")
	fmt.Fprintln(tw, "AGENT\tLAZYAGENTS\tPARTIAL\tFOREIGN\tFILE")
	for _, st := range statuses {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", st.AgentID, len(st.Enabled), len(st.Partial), st.Foreign, c.Paths.Tilde(st.File))
	}
	_ = tw.Flush()
	for _, p := range problems {
		fmt.Fprintln(c.Err, "lazyagents:", p)
	}
	for _, st := range statuses {
		if st.Note != "" {
			fmt.Fprintf(c.Err, "lazyagents: %s: %s\n", st.AgentID, st.Note)
		}
	}
	return 0
}

func hooksToggle(args []string, c cli.Context, svc *Service, enable bool) int {
	verb := "enable"
	if !enable {
		verb = "disable"
	}
	fs := cli.Flags("hooks "+verb, c.Err)
	agentID := fs.String("agent", "", "only this agent (default: every installed agent that supports it)")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintf(c.Err, "usage: lazyagents hooks %s <name> [--agent id]\n", verb)
		return 1
	}
	if !c.KnownAgent(*agentID) {
		return 1
	}
	var err error
	if enable {
		err = svc.Enable(name, *agentID)
	} else {
		err = svc.Disable(name, *agentID)
	}
	if err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintln(c.Out, toggleResult(name, *agentID, enable))
	for _, st := range svc.Status() {
		if st.Note != "" && enable && enabledIn(st, name) {
			fmt.Fprintf(c.Err, "lazyagents: %s: %s\n", st.AgentID, st.Note)
		}
	}
	return 0
}

func hooksAdd(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("hooks add", c.Err)
	event := fs.String("event", "", "event (SessionStart, PreToolUse, …)")
	command := fs.String("command", "", "command to run")
	matcher := fs.String("matcher", "", "event filter (regex, when the agent supports it)")
	timeout := fs.Int("timeout", 0, "timeout in seconds (0 = agent default)")
	desc := fs.String("desc", "", "description shown in the tab")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, `usage: lazyagents hooks add <name> --event SessionStart --command "..." [--matcher re] [--timeout n] [--desc text]`)
		return 1
	}
	h := Hook{Name: name, Description: *desc,
		Hooks: []agent.Hook{{Event: *event, Matcher: *matcher, Command: *command, Timeout: *timeout}}}
	if err := svc.Save(h); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	if p := CommandProblem(h); p != "" {
		fmt.Fprintln(c.Err, "lazyagents: warning:", p)
	}
	fmt.Fprintf(c.Out, "hook %q saved in %s\n", name, c.Paths.Tilde(svc.Dir()))
	return 0
}

func hooksRemove(args []string, c cli.Context, svc *Service) int {
	if len(args) != 1 {
		fmt.Fprintln(c.Err, "usage: lazyagents hooks rm <name>")
		return 1
	}
	if err := svc.Delete(args[0]); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "hook %q deleted from the library (agents where it is installed keep it)\n", args[0])
	return 0
}

func toggleResult(name, id string, enable bool) string {
	switch {
	case enable && id == "":
		return fmt.Sprintf("hook %q installed in every installed agent that supports it", name)
	case enable:
		return fmt.Sprintf("hook %q installed in %s", name, id)
	case id == "":
		return fmt.Sprintf("hook %q uninstalled from every installed agent that supports it", name)
	}
	return fmt.Sprintf("hook %q uninstalled from %s", name, id)
}

// firstArg tira o argumento posicional antes das flags e devolve o resto,
// para aceitar "hooks enable <nome> --agent id" (flag para de parsear no
// primeiro posicional).
func firstArg(fs *flag.FlagSet, args []string) (string, bool) {
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		return "", false
	}
	name := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return "", false
	}
	return name, true
}

// checks é a seção "hooks" do doctor: comando que não existe, arquivo de
// biblioteca inválido e o aviso de cada agente.
func checks(svc *Service) []cli.Check {
	return []cli.Check{{Title: "hooks", Run: func(c cli.Context, out io.Writer) []string {
		var problems []string
		lib, libProblems := svc.Library()
		for _, p := range libProblems {
			fmt.Fprintf(out, "  ✗ %s\n", p)
			problems = append(problems, "invalid hook: "+p)
		}
		for _, h := range lib {
			if p := CommandProblem(h); p != "" {
				fmt.Fprintf(out, "  ✗ %-16s %s\n", h.Name, p)
				problems = append(problems, "hook "+h.Name+": "+p)
			}
		}
		for _, st := range svc.Status() {
			switch {
			case st.Err != "":
				fmt.Fprintf(out, "  ✗ %-16s %s\n", st.AgentID, st.Err)
				problems = append(problems, "hooks in "+st.AgentID+": "+st.Err)
			default:
				line := fmt.Sprintf("  ✓ %-16s %d from lazyagents, %d foreign", st.AgentID, len(st.Enabled), st.Foreign)
				if len(st.Partial) > 0 {
					line = fmt.Sprintf("  ✓ %-16s %d from lazyagents, %d foreign, %d partial: %s", st.AgentID, len(st.Enabled), st.Foreign, len(st.Partial), strings.Join(st.Partial, ", "))
				}
				if st.Note != "" && len(st.Enabled) > 0 {
					line += "  — " + st.Note
				}
				fmt.Fprintln(out, line)
			}
		}
		return problems
	}}}
}
