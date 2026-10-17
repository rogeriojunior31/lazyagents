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
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// commands are the module's CLI subcommands.
func commands(svc *Service) []cli.Command {
	return []cli.Command{
		{Name: "provider", Usage: "provider list|validate <profile>|probe <profile>|models <profile>|apply <profile>|clear|adopt <profile>|add <profile>|rm <profile> [--agent id|--agents ids] [--preview] [--json] [--reveal]",
			Summary: "manage agent provider profiles (endpoint and token)",
			Help:    providerHelp,
			Run:     func(c cli.Context, a []string) int { return cmdProvider(a, c, svc) }},
	}
}

const providerHelp = `subcommands:
  list                 profiles and what each agent has applied (default)
  validate <profile>   check selected destinations without writing or network requests
  probe <profile>      send one credential-free HEAD request to the saved base URL
  models <profile>     fetch an OpenAI-compatible catalog using profile credentials
  apply <profile>      validate all selected targets, then write the profile to the agents' config
  clear                remove what lazyagents applied, keeping the rest of the file
  adopt <profile>      import public settings from --agent into the library only
  add <profile>        create or replace a profile
  rm <profile>         delete a profile from the library (agents keep what was applied)

options:
  --agent id           apply/validate/clear: only this agent (default: every installed agent that supports it)
  --agents ids         apply/validate: explicit comma-separated agent IDs
  --preview            apply/clear: inspect masked field changes without writing
  --json               list/validate/preview/probe/models: JSON output
  --reveal             list: show tokens in plain text

adopt options:
  --agent id           source agent (required); save public fields to a new library name
  --token -            explicitly re-enter a credential via stdin
  --env-key VAR        replace the imported credential reference

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
Pi and Crush need --base-url and --model. Validation checks configuration,
not authentication; a variable name does not prove a working credential.
Adopt copies public connection fields only. Re-enter native tokens with --token -
or supply --env-key; native account login and advanced options are not imported.
Apply/clear previews hide all values and never serialize private review revisions.
Any incompatible selected target blocks the entire apply before the first write.
Use --agent or --agents to explicitly select a compatible subset.
Probe/models are on-demand only, with a 10-second timeout and no redirects.
Models uses a saved bearer token before envKey; native login is never read.
A response proves reachability; a listed model does not prove inference works.
List compares public fields using native defaults and credential references/presence.
Multiple matching profiles stay ambiguous; token values are never compared.
List --json adds matches and per-profile comparisons; profile requires a unique match.
Start a new agent session after applying; existing sessions may keep old settings.
`

func cmdProvider(args []string, c cli.Context, svc *Service) int {
	sub := ""
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "list", "":
		return providerList(args, c, svc)
	case "probe", "models":
		return providerConnection(sub, args, c, svc)
	case "validate":
		return providerValidate(args, c, svc)
	case "apply":
		return providerApply(args, c, svc)
	case "clear":
		return providerClear(args, c, svc)
	case "adopt":
		return providerAdopt(args, c, svc)
	case "add":
		return providerAdd(args, c, svc)
	case "rm":
		return providerRemove(args, c, svc)
	default:
		fmt.Fprintf(c.Err, "lazyagents provider: unknown subcommand %q (list, validate, probe, models, apply, clear, adopt, add, rm)\n", sub)
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
	AgentID      string              `json:"agent"`
	Name         string              `json:"name"`
	File         string              `json:"file"`
	Installed    bool                `json:"installed"`
	Active       bool                `json:"active"`
	Applied      *jsonProfile        `json:"applied,omitempty"`
	Profile      string              `json:"profile,omitempty"`
	Err          string              `json:"error,omitempty"`
	SavedChanged bool                `json:"saved_changed,omitempty"`
	External     bool                `json:"external,omitempty"`
	Matches      []string            `json:"matches,omitempty"`
	Comparisons  []ProfileComparison `json:"comparisons,omitempty"`
}

func providerList(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider list", c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	reveal := fs.Bool("reveal", false, "show profile tokens in plain text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fs.Usage()
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
				Active: st.Active, Profile: st.Profile, Err: st.Err, SavedChanged: st.SavedChanged, External: st.External, Matches: st.Matches, Comparisons: st.Comparisons}
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
		profile := dash(st.Profile)
		if len(st.Matches) > 1 {
			profile = "ambiguous: " + strings.Join(st.Matches, ", ")
		} else if st.SavedChanged {
			profile = "no public-field match"
		} else if st.External && st.Profile != "" {
			profile += " (external match)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", st.AgentID, applied, profile, c.Paths.Tilde(st.File))
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
	if err := fs.Parse(fs.Args()[1:]); err != nil || fs.NArg() != 0 {
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
	agentIDs := fs.String("agents", "", "comma-separated selection of agent IDs")
	preview := fs.Bool("preview", false, "preview changes without writing")
	jsonOut := fs.Bool("json", false, "JSON preview")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, "usage: lazyagents provider apply <profile> [--agent id|--agents id,id]")
		return 1
	}
	ids, err := providerSelection(fs, *agentID, *agentIDs)
	if err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	for _, id := range ids {
		if !c.KnownAgent(id) {
			return 1
		}
	}
	if *jsonOut && !*preview {
		fmt.Fprintln(c.Err, "--json requires --preview")
		return 1
	}
	plan := svc.PrepareSaved(name, ids)
	if *preview {
		return outputPreparation(c, plan, *jsonOut)
	}
	printPreparation(c.Err, plan)
	if !plan.valid() {
		fmt.Fprintln(c.Err, "No files changed. Fix incompatible targets, or explicitly select a valid subset with --agent or --agents.")
		return 1
	}
	result := svc.ApplyReviewed(name, ids, plan)
	cli.PrintOperation(c.Err, result, "lazyagents config-backups list; docs/guide/recovery.md")
	if result.AsError() != nil {
		return 1
	}
	destination := inAgent(*agentID)
	if *agentIDs != "" {
		destination = strings.Join(ids, ", ")
	}
	fmt.Fprintf(c.Out, "profile %q applied to %s\n", name, destination)
	return 0
}

func providerClear(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider clear", c.Err)
	agentID := fs.String("agent", "", "only this agent (default: every installed agent that supports it)")
	preview := fs.Bool("preview", false, "preview changes without writing")
	jsonOut := fs.Bool("json", false, "JSON preview")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 1
	}
	if !c.KnownAgent(*agentID) {
		return 1
	}
	if *jsonOut && !*preview {
		fmt.Fprintln(c.Err, "--json requires --preview")
		return 1
	}
	ids := core.TargetIDs(*agentID)
	plan := svc.PrepareClear(ids)
	if *preview {
		return outputPreparation(c, plan, *jsonOut)
	}
	printPreparation(c.Err, plan)
	if !plan.valid() {
		fmt.Fprintln(c.Err, "No files changed. Fix the selected targets before clearing.")
		return 1
	}
	result := svc.ClearReviewed(ids, plan)
	cli.PrintOperation(c.Err, result, "lazyagents config-backups list; docs/guide/recovery.md")
	if result.AsError() != nil {
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
	if err := readProviderToken(c, token); err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
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
				switch {
				case len(st.Matches) > 1:
					name = "ambiguous public-field matches: " + strings.Join(st.Matches, ", ")
				case st.SavedChanged:
					name = "no saved profile matches public fields"
				case name == "":
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

func providerSelection(fs *flag.FlagSet, id, ids string) ([]string, error) {
	if id != "" && ids != "" {
		return nil, fmt.Errorf("choose either --agent or --agents")
	}
	agentsSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "agents" {
			agentsSet = true
		}
	})
	if agentsSet && ids == "" {
		return nil, fmt.Errorf("--agents needs a nonempty comma-separated list")
	}
	if ids == "" {
		return core.TargetIDs(id), nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, part := range strings.Split(ids, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("--agents needs a nonempty comma-separated list")
		}
		if !seen[part] {
			out = append(out, part)
			seen[part] = true
		}
	}
	return out, nil
}

func providerValidate(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider validate", c.Err)
	id := fs.String("agent", "", "only this agent")
	ids := fs.String("agents", "", "comma-separated selection of agent IDs")
	jsonOut := fs.Bool("json", false, "JSON output")
	name, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(c.Err, "usage: lazyagents provider validate <profile> [--agent id|--agents id,id] [--json]")
		return 1
	}
	selection, err := providerSelection(fs, *id, *ids)
	if err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	for _, target := range selection {
		if !c.KnownAgent(target) {
			return 1
		}
	}
	plan := svc.PrepareSaved(name, selection)
	if *jsonOut {
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(plan); err != nil {
			fmt.Fprintln(c.Err, "lazyagents:", err)
			return 1
		}
	} else {
		printPreparation(c.Out, plan)
	}
	if !plan.valid() {
		return 1
	}
	return 0
}

func printPreparation(out io.Writer, plan Preparation) {
	fmt.Fprintf(out, "Preview %s: %q (no writes, no network requests)\n", plan.Operation, plan.Profile)
	if plan.Error != "" {
		fmt.Fprintln(out, plan.Error)
	}
	for _, t := range plan.Targets {
		state := "ready"
		if t.Error != "" {
			state = "incompatible"
		}
		if t.Skipped {
			state = "skipped"
		}
		fmt.Fprintf(out, "%s: %s\n", t.ID, state)
		if t.Error != "" {
			fmt.Fprintln(out, "  "+t.Error)
		}
		if len(t.Preview.Files) == 0 {
			for _, file := range t.Files {
				fmt.Fprintln(out, "  "+file)
			}
		}
		for _, file := range t.Preview.Files {
			fmt.Fprintf(out, "  %s: %s\n", file.Action, file.Path)
			for _, change := range file.Changes {
				fmt.Fprintln(out, "    "+change)
			}
		}
		for _, note := range t.Notes {
			fmt.Fprintln(out, "  "+note)
		}
	}
}

func readProviderToken(c cli.Context, token *string) error {
	if *token != "-" {
		return nil
	}
	if c.In == nil {
		return fmt.Errorf("no stdin to read the token from")
	}
	data, err := io.ReadAll(io.LimitReader(c.In, (1<<16)+1))
	if err != nil {
		return fmt.Errorf("reading token: %w", err)
	}
	if len(data) > 1<<16 {
		return fmt.Errorf("token exceeds 64 KiB")
	}
	*token = strings.TrimSpace(string(data))
	return nil
}

func providerAdopt(args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider adopt", c.Err)
	id := fs.String("agent", "", "source agent (required)")
	token := fs.String("token", "", "explicit credential re-entry; - reads stdin")
	env := fs.String("env-key", "", "credential environment reference")
	name, ok := firstArg(fs, args)
	if !ok || *id == "" {
		fmt.Fprintln(c.Err, "usage: lazyagents provider adopt <profile> --agent id [--token -] [--env-key VAR]")
		return 1
	}
	if !c.KnownAgent(*id) {
		return 1
	}
	adoption, err := svc.AdoptDraft(*id)
	if err == nil {
		err = readProviderToken(c, token)
	}
	if err == nil {
		p := adoption.Profile
		p.Name, p.Token = name, *token
		if *env != "" {
			p.EnvKey = *env
		}
		err = svc.SaveAdoption(adoption, p)
	}
	if err != nil {
		fmt.Fprintln(c.Err, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(c.Out, "profile %q imported into the library; agent settings were not changed\n", name)
	return 0
}

func outputPreparation(c cli.Context, plan Preparation, jsonOut bool) int {
	if jsonOut {
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(plan); err != nil {
			fmt.Fprintln(c.Err, "lazyagents:", err)
			return 1
		}
	} else {
		printPreparation(c.Out, plan)
	}
	if !plan.valid() {
		return 1
	}
	return 0
}
