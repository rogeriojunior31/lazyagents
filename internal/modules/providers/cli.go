package providers

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
		{Name: "provider", Usage: "provider list|apply <perfil>|clear|add <perfil>|rm <perfil> [--agent id] [--json] [--reveal]",
			Run: func(c cli.Context, a []string) int { return cmdProvider(a, c, svc) }},
	}
}

func cmdProvider(args []string, c cli.Context, svc *Service) int {
	sub := ""
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "list", "":
		return providerList(args, c, svc)
	case "apply":
		return providerApply(args, c, svc)
	case "clear":
		return providerClear(args, c, svc)
	case "add":
		return providerAdd(args, c, svc)
	case "rm":
		return providerRemove(args, c, svc)
	default:
		fmt.Fprintf(c.Err, "lazyagents provider: subcomando desconhecido %q (list, apply, clear, add, rm)\n", sub)
		return 1
	}
}

func providerList(args []string, c cli.Context, svc *Service) int {
	fs := flag.NewFlagSet("provider list", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	jsonOut := fs.Bool("json", false, "saída JSON")
	reveal := fs.Bool("reveal", false, "mostra o token dos perfis em claro")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	profiles, err := svc.Profiles()
	if err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	statuses := svc.Status()

	if *jsonOut {
		// Sem --reveal o token não sai nem no JSON (regra 7).
		out := make([]agent.ProviderProfile, 0, len(profiles))
		for _, p := range profiles {
			if !*reveal {
				p = p.Redacted()
			}
			out = append(out, p)
		}
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Profiles []agent.ProviderProfile `json:"profiles"`
			Agents   []Status                `json:"agents"`
		}{out, statuses})
		return 0
	}

	tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PERFIL\tENDPOINT\tMODELO\tTOKEN")
	for _, p := range profiles {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Name, dash(p.BaseURL), dash(p.Model), tokenCell(p, *reveal))
	}
	if len(profiles) == 0 {
		fmt.Fprintln(tw, "(nenhum perfil)\t\t\t")
	}
	fmt.Fprintln(tw, "\t\t\t")
	fmt.Fprintln(tw, "AGENTE\tAPLICADO\tPERFIL\tARQUIVO")
	for _, st := range statuses {
		applied := "-"
		if st.Active {
			applied = dash(st.Applied.BaseURL)
		}
		if st.Err != "" {
			applied = st.Err
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", st.AgentID, applied, dash(st.Profile), c.Paths.Tilde(st.File))
	}
	_ = tw.Flush()
	return 0
}

// firstArg tira o argumento posicional antes das flags e devolve o resto,
// para aceitar "provider apply <perfil> --agent id" (flag para de parsear no
// primeiro posicional). Mesmo padrão de cmdToggle.
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

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// tokenCell mostra a presença do token; o valor só com --reveal explícito.
func tokenCell(p agent.ProviderProfile, reveal bool) string {
	switch {
	case p.Token == "" && p.EnvKey != "":
		return "$" + p.EnvKey
	case p.Token == "":
		return "-"
	case reveal:
		return p.Token
	default:
		return "••••"
	}
}

func providerApply(args []string, c cli.Context, svc *Service) int {
	fs := flag.NewFlagSet("provider apply", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	agentID := fs.String("agent", "", "só este agente (padrão: todos os instalados que suportam)")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, "uso: lazyagents provider apply <perfil> [--agent id]")
		return 1
	}
	if err := svc.Apply(name, *agentID); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "perfil %q aplicado%s\n", name, inAgent(*agentID))
	return 0
}

func providerClear(args []string, c cli.Context, svc *Service) int {
	fs := flag.NewFlagSet("provider clear", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	agentID := fs.String("agent", "", "só este agente (padrão: todos os instalados que suportam)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := svc.Clear(*agentID); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "provedor removido%s\n", inAgent(*agentID))
	return 0
}

func inAgent(id string) string {
	if id == "" {
		return " em todos os agentes instalados que suportam"
	}
	return " em " + id
}

func providerAdd(args []string, c cli.Context, svc *Service) int {
	fs := flag.NewFlagSet("provider add", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	baseURL := fs.String("base-url", "", "endpoint compatível")
	model := fs.String("model", "", "modelo padrão")
	token := fs.String("token", "", `token; "-" lê da entrada padrão (não fica no histórico do shell)`)
	envKey := fs.String("env-key", "", "nome da variável de ambiente com o token (Codex)")
	wireAPI := fs.String("wire-api", "", "Codex: responses")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, "uso: lazyagents provider add <perfil> [--base-url url] [--model m] [--token -] [--env-key VAR] [--wire-api responses]")
		return 1
	}
	if *token == "-" {
		if c.In == nil {
			fmt.Fprintln(c.Err, "lazyagents: sem entrada padrão para ler o token")
			return 1
		}
		data, err := io.ReadAll(io.LimitReader(c.In, 1<<16))
		if err != nil {
			fmt.Fprintln(c.Err, "lazyagents: lendo token:", err)
			return 1
		}
		*token = strings.TrimSpace(string(data))
	}
	p := agent.ProviderProfile{Name: name, BaseURL: *baseURL, Model: *model, Token: *token, EnvKey: *envKey, WireAPI: *wireAPI}
	if err := svc.Save(p); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "perfil %q salvo em %s\n", p.Name, c.Paths.Tilde(svc.Path()))
	return 0
}

func providerRemove(args []string, c cli.Context, svc *Service) int {
	if len(args) != 1 {
		fmt.Fprintln(c.Err, "uso: lazyagents provider rm <perfil>")
		return 1
	}
	if err := svc.Delete(args[0]); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "perfil %q removido (os agentes onde ele foi aplicado não foram tocados)\n", args[0])
	return 0
}

// checks reporta no doctor o provedor aplicado em cada agente.
func checks(svc *Service) []cli.Check {
	return []cli.Check{{Title: "provedores", Run: func(c cli.Context, out io.Writer) []string {
		var problems []string
		for _, st := range svc.Status() {
			switch {
			case st.Err != "":
				fmt.Fprintf(out, "  ✗ %-16s %s\n", st.AgentID, st.Err)
				problems = append(problems, "provedor de "+st.AgentID+": "+st.Err)
			case st.Active && !st.Installed:
				fmt.Fprintf(out, "  ✗ %-16s provedor aplicado num agente que não está instalado\n", st.AgentID)
				problems = append(problems, st.AgentID+": provedor aplicado, mas o agente não está instalado")
			case st.Active:
				name := st.Profile
				if name == "" {
					name = "fora do lazyagents"
				}
				fmt.Fprintf(out, "  ✓ %-16s %s (%s)\n", st.AgentID, dash(st.Applied.BaseURL), name)
			default:
				fmt.Fprintf(out, "  ✓ %-16s padrão do agente\n", st.AgentID)
			}
		}
		return problems
	}}}
}
