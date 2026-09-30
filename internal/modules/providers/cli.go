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

// commands are the module's CLI subcommands.
func commands(svc *Service) []cli.Command {
	return []cli.Command{
		{Name: "provider", Usage: "provider list|apply <profile>|clear|add <profile>|rm <profile> [--agent id] [--json] [--reveal]",
			Summary: "manage agent provider profiles (endpoint and token)",
			Help:    providerHelp,
			Run:     func(c cli.Context, a []string) int { return cmdProvider(a, c, svc) }},
	}
}

const providerHelp = `subcommands:
  list                 profiles and what each agent has applied (default)
  apply <profile>      write the profile to the agents' config
  clear                remove what lazyagents applied, keeping the rest of the file
  add <profile>        create or replace a profile
  rm <profile>         delete a profile from the library (agents keep what was applied)

options:
  --agent id           apply/clear: only this agent (default: every installed agent that supports it)
  --json               list: JSON output
  --reveal             list: show tokens in plain text

add options:
  --base-url url       compatible endpoint (http or https)
  --model m            default model (empty = agent default)
  --token -            read the token from stdin; a literal value also works but lands in shell history
  --env-key VAR        Codex, Pi, Crush: environment variable holding the token
  --wire-api api       Codex: only "responses"; Pi: chat (default), responses or anthropic;
                       Crush: chat (default) or anthropic

Profiles live in providers.json with mode 0600. Tokens are masked in text and
JSON output unless --reveal is given. Every apply and clear backs the agent's
file up first. Codex never gets the token: set --env-key and export that variable.
Pi and Crush need --base-url and --model.
`

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
		fmt.Fprintf(c.Err, "lazyagents provider: unknown subcommand %q (list, apply, clear, add, rm)\n", sub)
		return 1
	}
}

// jsonProfile is the --json shape of a profile: snake_case like every other
// command's output. providers.json keeps its own (camelCase) format on disk.
type jsonProfile struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url,omitempty"`
	Model    string `json:"model,omitempty"`
	Token    string `json:"token,omitempty"` // only with --reveal
	HasToken bool   `json:"has_token"`
	EnvKey   string `json:"env_key,omitempty"`
	WireAPI  string `json:"wire_api,omitempty"`
}

func toJSONProfile(p agent.ProviderProfile) jsonProfile {
	return jsonProfile{Name: p.Name, BaseURL: p.BaseURL, Model: p.Model, Token: p.Token,
		HasToken: p.HasToken || p.Token != "", EnvKey: p.EnvKey, WireAPI: p.WireAPI}
}

type jsonStatus struct {
	AgentID   string       `json:"agent"`
	Name      string       `json:"name"`
	File      string       `json:"file"`
	Installed bool         `json:"installed"`
	Active    bool         `json:"active"`
	Applied   *jsonProfile `json:"applied,omitempty"`
	Profile   string       `json:"profile,omitempty"`
	Err       string       `json:"error,omitempty"`
}

func providerList(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider list", c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	reveal := fs.Bool("reveal", false, "show profile tokens in plain text")
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
		// Without --reveal the token stays out of JSON too (rule 7).
		out := make([]jsonProfile, 0, len(profiles))
		for _, p := range profiles {
			if !*reveal {
				p = p.Redacted()
			}
			out = append(out, toJSONProfile(p))
		}
		agents := make([]jsonStatus, 0, len(statuses))
		for _, st := range statuses {
			j := jsonStatus{AgentID: st.AgentID, Name: st.AgentName, File: st.File, Installed: st.Installed,
				Active: st.Active, Profile: st.Profile, Err: st.Err}
			if st.Applied != (agent.ProviderProfile{}) {
				applied := toJSONProfile(st.Applied.Redacted())
				j.Applied = &applied
			}
			agents = append(agents, j)
		}
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Profiles []jsonProfile `json:"profiles"`
			Agents   []jsonStatus  `json:"agents"`
		}{out, agents})
		return 0
	}

	tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROFILE\tENDPOINT\tMODEL\tTOKEN")
	for _, p := range profiles {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Name, dash(p.BaseURL), dash(p.Model), tokenCell(p, *reveal))
	}
	if len(profiles) == 0 {
		fmt.Fprintln(tw, "(no profiles)\t\t\t")
	}
	fmt.Fprintln(tw, "\t\t\t")
	fmt.Fprintln(tw, "AGENT\tAPPLIED\tPROFILE\tFILE")
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

// firstArg pulls the positional argument out before the flags, so
// "provider apply <profile> --agent id" works (flag stops at the first
// positional). Same pattern as cmdToggle.
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

// tokenCell shows whether there is a token; the value only with --reveal.
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
	fs := cli.Flags("provider apply", c.Err)
	agentID := fs.String("agent", "", "only this agent (default: every installed agent that supports it)")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, "usage: lazyagents provider apply <profile> [--agent id]")
		return 1
	}
	if !c.KnownAgent(*agentID) {
		return 1
	}
	if err := svc.Apply(name, *agentID); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "profile %q applied to %s\n", name, inAgent(*agentID))
	return 0
}

func providerClear(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider clear", c.Err)
	agentID := fs.String("agent", "", "only this agent (default: every installed agent that supports it)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !c.KnownAgent(*agentID) {
		return 1
	}
	if err := svc.Clear(*agentID); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "provider cleared from %s\n", inAgent(*agentID))
	return 0
}

func inAgent(id string) string {
	if id == "" {
		return "every installed agent that supports it"
	}
	return id
}

func providerAdd(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider add", c.Err)
	baseURL := fs.String("base-url", "", "compatible endpoint")
	model := fs.String("model", "", "default model")
	token := fs.String("token", "", `token; "-" reads it from stdin (keeps it out of shell history)`)
	envKey := fs.String("env-key", "", "name of the environment variable holding the token (Codex, Pi)")
	wireAPI := fs.String("wire-api", "", "Codex: responses; Pi: chat, responses or anthropic; Crush: chat or anthropic")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, "usage: lazyagents provider add <profile> [--base-url url] [--model m] [--token -] [--env-key VAR] [--wire-api responses]")
		return 1
	}
	if *token == "-" {
		if c.In == nil {
			fmt.Fprintln(c.Err, "lazyagents: no stdin to read the token from")
			return 1
		}
		data, err := io.ReadAll(io.LimitReader(c.In, 1<<16))
		if err != nil {
			fmt.Fprintln(c.Err, "lazyagents: reading token:", err)
			return 1
		}
		*token = strings.TrimSpace(string(data))
	}
	p := agent.ProviderProfile{Name: name, BaseURL: *baseURL, Model: *model, Token: *token, EnvKey: *envKey, WireAPI: *wireAPI}
	if err := svc.Save(p); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "profile %q saved to %s\n", p.Name, c.Paths.Tilde(svc.Path()))
	return 0
}

func providerRemove(args []string, c cli.Context, svc *Service) int {
	if len(args) != 1 {
		fmt.Fprintln(c.Err, "usage: lazyagents provider rm <profile>")
		return 1
	}
	if err := svc.Delete(args[0]); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "profile %q removed (agents where it was applied were not touched)\n", args[0])
	return 0
}

// checks reports the provider applied in each agent to doctor.
func checks(svc *Service) []cli.Check {
	return []cli.Check{{Title: "providers", Run: func(c cli.Context, out io.Writer) []string {
		var problems []string
		for _, st := range svc.Status() {
			switch {
			case st.Err != "":
				fmt.Fprintf(out, "  ✗ %-16s %s\n", st.AgentID, st.Err)
				problems = append(problems, fmt.Sprintf("provider for %s: %s", st.AgentID, st.Err))
			case st.Active && !st.Installed:
				fmt.Fprintf(out, "  ✗ %-16s provider applied to an agent that is not installed\n", st.AgentID)
				problems = append(problems, fmt.Sprintf("%s: provider applied, but the agent is not installed", st.AgentID))
			case st.Active:
				name := st.Profile
				if name == "" {
					name = "outside lazyagents"
				}
				fmt.Fprintf(out, "  ✓ %-16s %s (%s)\n", st.AgentID, dash(st.Applied.BaseURL), name)
			default:
				fmt.Fprintf(out, "  ✓ %-16s agent default\n", st.AgentID)
			}
		}
		return problems
	}}}
}
