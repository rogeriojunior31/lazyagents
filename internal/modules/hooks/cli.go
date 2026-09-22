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
		{Name: "hooks", Usage: "hooks list|enable <nome>|disable <nome>|add <nome>|rm <nome> [--agent id] [--json]",
			Run: func(c cli.Context, a []string) int { return cmdHooks(a, c, svc) }},
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
		fmt.Fprintf(c.Err, "lazyagents hooks: subcomando desconhecido %q (list, enable, disable, add, rm)\n", sub)
		return 1
	}
}

func hooksList(args []string, c cli.Context, svc *Service) int {
	fs := flag.NewFlagSet("hooks list", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	jsonOut := fs.Bool("json", false, "saída JSON")
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
	fmt.Fprintln(tw, "HOOK\tEVENTOS\tCOMANDO\tINSTALADO EM")
	for _, h := range lib {
		var in []string
		for _, st := range statuses {
			if enabledIn(st, h.Name) {
				in = append(in, st.AgentID)
			} else if partialIn(st, h.Name) {
				in = append(in, st.AgentID+" (parcial)")
			}
		}
		where := "-"
		if len(in) > 0 {
			where = strings.Join(in, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.Name, strings.Join(h.Events(), ","), h.Summary(), where)
	}
	if len(lib) == 0 {
		fmt.Fprintln(tw, "(biblioteca vazia)\t\t\t")
	}
	fmt.Fprintln(tw, "\t\t\t\t")
	fmt.Fprintln(tw, "AGENTE\tDO LAZYAGENTS\tPARCIAIS\tPRÓPRIOS\tARQUIVO")
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
	fs := flag.NewFlagSet("hooks "+verb, flag.ContinueOnError)
	fs.SetOutput(c.Err)
	agentID := fs.String("agent", "", "só este agente (padrão: todos os instalados que suportam)")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintf(c.Err, "uso: lazyagents hooks %s <nome> [--agent id]\n", verb)
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
	action := "instalado"
	if !enable {
		action = "removido"
	}
	fmt.Fprintf(c.Out, "hook %q %s%s\n", name, action, inAgent(*agentID))
	for _, st := range svc.Status() {
		if st.Note != "" && enable && enabledIn(st, name) {
			fmt.Fprintf(c.Err, "lazyagents: %s: %s\n", st.AgentID, st.Note)
		}
	}
	return 0
}

func hooksAdd(args []string, c cli.Context, svc *Service) int {
	fs := flag.NewFlagSet("hooks add", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	event := fs.String("event", "", "evento (SessionStart, PreToolUse, …)")
	command := fs.String("command", "", "comando a rodar")
	matcher := fs.String("matcher", "", "filtro do evento (regex, quando o agente suporta)")
	timeout := fs.Int("timeout", 0, "timeout em segundos (0 = default do agente)")
	desc := fs.String("desc", "", "descrição para a aba")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, `uso: lazyagents hooks add <nome> --event SessionStart --command "..." [--matcher re] [--timeout n] [--desc texto]`)
		return 1
	}
	h := Hook{Name: name, Description: *desc,
		Hooks: []agent.Hook{{Event: *event, Matcher: *matcher, Command: *command, Timeout: *timeout}}}
	if err := svc.Save(h); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	if p := CommandProblem(h); p != "" {
		fmt.Fprintln(c.Err, "lazyagents: atenção:", p)
	}
	fmt.Fprintf(c.Out, "hook %q salvo em %s\n", name, c.Paths.Tilde(svc.Dir()))
	return 0
}

func hooksRemove(args []string, c cli.Context, svc *Service) int {
	if len(args) != 1 {
		fmt.Fprintln(c.Err, "uso: lazyagents hooks rm <nome>")
		return 1
	}
	if err := svc.Delete(args[0]); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "hook %q apagado da biblioteca (onde já estava instalado, continua)\n", args[0])
	return 0
}

func inAgent(id string) string {
	if id == "" {
		return " em todos os agentes instalados que suportam"
	}
	return " em " + id
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
			problems = append(problems, "hook inválido: "+p)
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
				problems = append(problems, "hooks de "+st.AgentID+": "+st.Err)
			default:
				line := fmt.Sprintf("  ✓ %-16s %d do lazyagents, %d próprio(s)", st.AgentID, len(st.Enabled), st.Foreign)
				if len(st.Partial) > 0 {
					line += fmt.Sprintf(", %d parcial(is): %s", len(st.Partial), strings.Join(st.Partial, ", "))
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
