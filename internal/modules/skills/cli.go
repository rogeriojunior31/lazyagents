package skills

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

// commands are this module's CLI subcommands.
func commands(svc *Service) []cli.Command {
	cmds := []cli.Command{
		{Name: "list", Usage: "list [--json]", Summary: "list skills and how many agents have them enabled",
			Run: func(c cli.Context, a []string) int { return cmdList(a, c.Out, c.Err, svc, c.Agents()) }},
		{Name: "enable", Usage: "enable <skill> [--agent id|--all]", Summary: "enable a library skill (symlink in the agent)",
			Run: func(c cli.Context, a []string) int { return cmdToggle(a, c.Out, c.Err, svc, c.Agents(), true) }},
		{Name: "disable", Usage: "disable <skill> [--agent id|--all]", Summary: "disable a skill (removes the symlink)",
			Run: func(c cli.Context, a []string) int { return cmdToggle(a, c.Out, c.Err, svc, c.Agents(), false) }},
		{Name: "install", Usage: "install <source> [--hooks]", Summary: "install skills from a GitHub repo, zip or directory",
			Run: func(c cli.Context, a []string) int { return cmdInstall(a, c.Out, c.Err, svc) }},
		{Name: "remove", Usage: "remove <skill>", Summary: "remove a skill from the library",
			Run: func(c cli.Context, a []string) int { return cmdRemove(a, c.Out, c.Err, svc, c.Agents()) }},
		{Name: "adopt", Usage: "adopt <skill> --agent <id>", Summary: "move an agent's local skill into the library",
			Run: func(c cli.Context, a []string) int { return cmdAdopt(a, c.Out, c.Err, svc, c.Agents()) }},
		{Name: "migrate-library", Usage: "migrate-library <dir>", Summary: "move the skills library to another directory",
			Run: func(c cli.Context, a []string) int { return cmdMigrateLibrary(a, c, svc) }},
	}
	// `skills <sub>` groups the same commands, like hooks and provider.
	group := cli.Command{Name: "skills", Usage: "skills list|enable|disable|install|remove|adopt|migrate-library …",
		Summary: "the skill commands above, grouped (skills list = list)",
		Run: func(c cli.Context, a []string) int {
			if len(a) == 0 {
				a = []string{"list"}
			}
			for _, cmd := range cmds {
				if cmd.Name == a[0] {
					return cmd.Run(c, a[1:])
				}
			}
			fmt.Fprintf(c.Err, "lazyagents skills: unknown subcommand %q (list, enable, disable, install, remove, adopt, migrate-library)\n", a[0])
			return 1
		}}
	return append(cmds, group)
}

// --- JSON output types ---

type jsonOrigin struct {
	Type   string `json:"type"`
	Source string `json:"source,omitempty"`
	Sub    string `json:"sub,omitempty"`
}

type jsonAgentState struct {
	AgentID string `json:"agent_id"`
	On      bool   `json:"on"`
	Managed bool   `json:"managed"`
	Local   bool   `json:"local"`
}

type jsonSkillItem struct {
	Dir         string           `json:"dir"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InLibrary   bool             `json:"in_library"`
	Valid       bool             `json:"valid"`
	Origin      *jsonOrigin      `json:"origin,omitempty"`
	States      []jsonAgentState `json:"states"`
}

// --- helpers ---

func withSkills(errOut io.Writer, skillSvc *Service, agents []agent.Agent) ([]Skill, bool) {
	skills, err := skillSvc.Scan(agents)
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return nil, false
	}
	return skills, true
}

func findSkill(name string, skills []Skill) (Skill, bool) {
	for _, sk := range skills {
		if sk.Dir == name || sk.Name == name {
			return sk, true
		}
	}
	return Skill{}, false
}

func findAgent(id string, agents []agent.Agent) (agent.Agent, bool) {
	for _, ag := range agents {
		if ag.ID == id {
			return ag, true
		}
	}
	return agent.Agent{}, false
}

func toJSONSkill(sk Skill) jsonSkillItem {
	item := jsonSkillItem{
		Dir:         sk.Dir,
		Name:        sk.Name,
		Description: sk.Description,
		InLibrary:   sk.InLibrary,
		Valid:       sk.Valid,
	}
	if sk.Origin != nil {
		item.Origin = &jsonOrigin{Type: sk.Origin.Type, Source: sk.Origin.Source, Sub: sk.Origin.Sub}
	}
	for id, st := range sk.States {
		item.States = append(item.States, jsonAgentState{AgentID: id, On: st.On, Managed: st.Managed, Local: st.Local})
	}
	return item
}

func cmdList(args []string, out, errOut io.Writer, skillSvc *Service, agents []agent.Agent) int {
	fs := cli.Flags("list", errOut)
	jsonOut := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	if *jsonOut {
		items := make([]jsonSkillItem, 0, len(skills))
		for _, sk := range skills {
			items = append(items, toJSONSkill(sk))
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(items)
		return 0
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SKILL\tDESCRIPTION\tENABLED IN\tLIBRARY")
	for _, sk := range skills {
		lib := ""
		if sk.InLibrary {
			lib = "yes"
		}
		// the full description is in --json; here it would break the table
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", sk.Dir, oneLine(sk.Description, 60), sk.EnabledCount(), lib)
	}
	return func() int {
		if err := tw.Flush(); err != nil {
			fmt.Fprintln(errOut, "lazyagents:", err)
			return 1
		}
		return 0
	}()
}

func cmdToggle(args []string, out, errOut io.Writer, skillSvc *Service, agents []agent.Agent, enable bool) int {
	verb := "enable"
	if !enable {
		verb = "disable"
	}
	fs := cli.Flags(verb, errOut)
	agentID := fs.String("agent", "", "agent ID")
	all := fs.Bool("all", false, "all installed agents")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		fmt.Fprintf(errOut, "usage: lazyagents %s <skill> [--agent id|--all]\n", verb)
		return 1
	}
	name := fs.Arg(0)
	// flag stops at the first positional arg; reparse the rest so
	// "enable <skill> --agent id" works.
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(name, skills)
	if !found {
		fmt.Fprintf(errOut, "lazyagents: skill %q not found\n", name)
		return 1
	}

	targets := agents
	if *agentID != "" {
		ag, ok := findAgent(*agentID, agents)
		if !ok {
			fmt.Fprintf(errOut, "lazyagents: agent %q not found\n", *agentID)
			return 1
		}
		targets = []agent.Agent{ag}
	} else if !*all {
		// no flag: all agents (same as --all)
		targets = agents
	}

	var errs []string
	for _, ag := range targets {
		var err error
		if enable {
			err = skillSvc.Enable(sk, ag)
		} else {
			err = skillSvc.Disable(sk, ag)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ag.ID, err))
		}
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(errOut, "lazyagents:", e)
		}
		return 1
	}
	if enable {
		fmt.Fprintf(out, "skill %q enabled\n", name)
	} else {
		fmt.Fprintf(out, "skill %q disabled\n", name)
	}
	return 0
}

func cmdInstall(args []string, out, errOut io.Writer, skillSvc *Service) int {
	fs := cli.Flags("install", errOut)
	withHooks := fs.Bool("hooks", false, "also install the source's plugin hooks")
	source, ok := firstArg(fs, args)
	if !ok {
		fmt.Fprintln(errOut, "usage: lazyagents install <source> [--hooks]")
		return 1
	}
	found, origin, cleanup, err := skillSvc.Discover(source)
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	for _, n := range origin.Notes {
		fmt.Fprintln(errOut, "lazyagents:", n)
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	// Hooks run third-party commands on every event: without --hooks, only the
	// skills are installed (the TUI leaves hooks unchecked too).
	if !*withHooks {
		var skills []Found
		pending := 0
		for _, f := range found {
			if f.Hook != nil {
				pending++
				continue
			}
			skills = append(skills, f)
		}
		if pending > 0 {
			fmt.Fprintf(errOut, "lazyagents: %d plugin hook(s) in this source; use --hooks to install them too\n", pending)
		}
		found = skills
	}
	names, err := skillSvc.Install(found, origin)
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	for _, n := range names {
		fmt.Fprintf(out, "installed: %s\n", n)
	}
	return 0
}

func cmdRemove(args []string, out, errOut io.Writer, skillSvc *Service, agents []agent.Agent) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: lazyagents remove <skill>")
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(args[0], skills)
	if !found {
		fmt.Fprintf(errOut, "lazyagents: skill %q not found\n", args[0])
		return 1
	}
	if err := skillSvc.Remove(sk, agents); err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(out, "skill %q removed\n", args[0])
	return 0
}

func cmdAdopt(args []string, out, errOut io.Writer, skillSvc *Service, agents []agent.Agent) int {
	fs := cli.Flags("adopt", errOut)
	agentID := fs.String("agent", "", "source agent ID (required)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(errOut, "usage: lazyagents adopt <skill> --agent <id>")
		return 1
	}
	name := fs.Arg(0)
	// flag stops at the first positional arg; reparse the rest so
	// "adopt <skill> --agent id" works.
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 1
	}
	if *agentID == "" {
		fmt.Fprintln(errOut, "usage: lazyagents adopt <skill> --agent <id>")
		return 1
	}
	ag, ok := findAgent(*agentID, agents)
	if !ok {
		fmt.Fprintf(errOut, "lazyagents: agent %q not found\n", *agentID)
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(name, skills)
	if !found {
		fmt.Fprintf(errOut, "lazyagents: skill %q not found\n", name)
		return 1
	}
	if err := skillSvc.Adopt(sk, ag); err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(out, "skill %q adopted into the library\n", name)
	return 0
}

func cmdMigrateLibrary(args []string, c cli.Context, skillSvc *Service) int {
	out, errOut, agents := c.Out, c.Err, c.Agents()
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: lazyagents migrate-library <dir>")
		return 1
	}
	newDir := c.Paths.ExpandHome(args[0])
	fmt.Fprintf(out, "migrating library to %s...\n", newDir)
	if err := skillSvc.MigrateLibrary(newDir, agents); err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	fmt.Fprintln(out, "migration complete")
	return 0
}

// checks are this module's doctor sections.
func checks(svc *Service) []cli.Check {
	return []cli.Check{
		{Title: "skills", Run: func(c cli.Context, out io.Writer) []string {
			skills, err := svc.Scan(c.Agents())
			if err != nil {
				fmt.Fprintln(c.Err, "lazyagents:", err)
				return []string{err.Error()}
			}
			var problems []string
			for _, sk := range skills {
				if !sk.Valid {
					problems = append(problems, fmt.Sprintf("skill %q: %s", sk.Dir, sk.Warning))
					fmt.Fprintf(out, "  %-30s WARNING: %s\n", sk.Dir, sk.Warning)
					continue
				}
				issues := Validate(sk)
				if len(issues) == 0 {
					fmt.Fprintf(out, "  %-30s OK\n", sk.Dir)
					continue
				}
				fmt.Fprintf(out, "  %-30s WARNING:\n", sk.Dir)
				for _, iss := range issues {
					problems = append(problems, fmt.Sprintf("skill %q: %s: %s", sk.Dir, iss.Field, iss.Msg))
					fmt.Fprintf(out, "    - %s: %s\n", iss.Field, iss.Msg)
				}
			}
			return problems
		}},
		{Title: "symlinks", Run: func(c cli.Context, out io.Writer) []string {
			var problems []string
			for _, ag := range c.Agents() {
				for _, dir := range ag.ReadDirs {
					entries, err := os.ReadDir(dir)
					if err != nil {
						continue
					}
					for _, e := range entries {
						path := filepath.Join(dir, e.Name())
						info, err := os.Lstat(path)
						if err != nil || info.Mode()&os.ModeSymlink == 0 {
							continue
						}
						if _, err := os.Stat(path); os.IsNotExist(err) {
							msg := fmt.Sprintf("broken symlink: %s", path)
							problems = append(problems, msg)
							fmt.Fprintf(out, "  WARNING: %s\n", msg)
						}
					}
				}
			}
			if len(problems) == 0 {
				fmt.Fprintln(out, "  all OK")
			}
			return problems
		}},
	}
}

// firstArg splits off the positional argument before the flags, so
// "install <source> --hooks" works (flag stops at the first positional).
func firstArg(fs *flag.FlagSet, args []string) (string, bool) {
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		return "", false
	}
	first := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return "", false
	}
	return first, true
}

// oneLine joins s's lines and cuts it at n runes, with an ellipsis.
func oneLine(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
