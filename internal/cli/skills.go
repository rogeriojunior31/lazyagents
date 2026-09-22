package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
)

// SkillCommands são os subcomandos da feature skills.
func SkillCommands(svc *skill.Service) []Command {
	return []Command{
		{Name: "list", Usage: "list [--json]", Run: func(c Context, a []string) int {
			return cmdList(a, c.Out, c.Err, svc, c.Agents())
		}},
		{Name: "enable", Usage: "enable <skill> [--agent id|--all]", Run: func(c Context, a []string) int {
			return cmdToggle(a, c.Out, c.Err, svc, c.Agents(), true)
		}},
		{Name: "disable", Usage: "disable <skill> [--agent id|--all]", Run: func(c Context, a []string) int {
			return cmdToggle(a, c.Out, c.Err, svc, c.Agents(), false)
		}},
		{Name: "install", Usage: "install <origem>", Run: func(c Context, a []string) int {
			return cmdInstall(a, c.Out, c.Err, svc)
		}},
		{Name: "remove", Usage: "remove <skill>", Run: func(c Context, a []string) int {
			return cmdRemove(a, c.Out, c.Err, svc, c.Agents())
		}},
		{Name: "adopt", Usage: "adopt <skill> --agent <id>", Run: func(c Context, a []string) int {
			return cmdAdopt(a, c.Out, c.Err, svc, c.Agents())
		}},
		{Name: "migrate-library", Usage: "migrate-library <dir>", Run: func(c Context, a []string) int {
			return cmdMigrateLibrary(a, c, svc)
		}},
	}
}

// --- tipos para saída JSON ---

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

type jsonSessionItem struct {
	ID      string     `json:"id"`
	Agent   string     `json:"agent"`
	Title   string     `json:"title"`
	CWD     string     `json:"cwd,omitempty"`
	Updated string     `json:"updated,omitempty"`
	Usage   *jsonUsage `json:"usage,omitempty"`
}

// jsonUsage só aparece quando o adapter da sessão sabe informar tokens
// (agent.UsageReader).
type jsonUsage struct {
	Input      int      `json:"input"`
	Output     int      `json:"output"`
	CacheRead  int      `json:"cache_read"`
	CacheWrite int      `json:"cache_write"`
	Model      string   `json:"model,omitempty"`
	CostUSD    *float64 `json:"cost_usd,omitempty"`
}

// --- helpers ---

func withSkills(errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) ([]skill.Skill, bool) {
	skills, err := skillSvc.Scan(agents)
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return nil, false
	}
	return skills, true
}

func findSkill(name string, skills []skill.Skill) (skill.Skill, bool) {
	for _, sk := range skills {
		if sk.Dir == name || sk.Name == name {
			return sk, true
		}
	}
	return skill.Skill{}, false
}

func findAgent(id string, agents []agent.Agent) (agent.Agent, bool) {
	for _, ag := range agents {
		if ag.ID == id {
			return ag, true
		}
	}
	return agent.Agent{}, false
}

func toJSONSkill(sk skill.Skill) jsonSkillItem {
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

func cmdList(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonOut := fs.Bool("json", false, "saída JSON")
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
	fmt.Fprintln(tw, "SKILL\tDESCRIÇÃO\tATIVA EM\tBIBLIOTECA")
	for _, sk := range skills {
		lib := ""
		if sk.InLibrary {
			lib = "sim"
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", sk.Dir, sk.Description, sk.EnabledCount(), lib)
	}
	return func() int {
		if err := tw.Flush(); err != nil {
			fmt.Fprintln(errOut, "lazyagents:", err)
			return 1
		}
		return 0
	}()
}

func cmdToggle(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent, enable bool) int {
	verb := "enable"
	if !enable {
		verb = "disable"
	}
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(errOut)
	agentID := fs.String("agent", "", "ID do agente")
	all := fs.Bool("all", false, "todos os agentes instalados")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		fmt.Fprintf(errOut, "uso: lazyagents %s <skill> [--agent id|--all]\n", verb)
		return 1
	}
	name := fs.Arg(0)
	// flag para de parsear no primeiro arg posicional; re-parseia o resto
	// para aceitar "enable <skill> --agent id".
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(name, skills)
	if !found {
		fmt.Fprintf(errOut, "lazyagents: skill %q não encontrada\n", name)
		return 1
	}

	targets := agents
	if *agentID != "" {
		ag, ok := findAgent(*agentID, agents)
		if !ok {
			fmt.Fprintf(errOut, "lazyagents: agente %q não encontrado\n", *agentID)
			return 1
		}
		targets = []agent.Agent{ag}
	} else if !*all {
		// sem flag: aplica a todos (mesmo comportamento de --all)
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
		fmt.Fprintf(out, "skill %q ativada\n", name)
	} else {
		fmt.Fprintf(out, "skill %q desativada\n", name)
	}
	return 0
}

func cmdInstall(args []string, out, errOut io.Writer, skillSvc *skill.Service) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: lazyagents install <origem>")
		return 1
	}
	found, origin, cleanup, err := skillSvc.Discover(args[0])
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
	names, err := skillSvc.Install(found, origin)
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	for _, n := range names {
		fmt.Fprintf(out, "instalada: %s\n", n)
	}
	return 0
}

func cmdRemove(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: lazyagents remove <skill>")
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(args[0], skills)
	if !found {
		fmt.Fprintf(errOut, "lazyagents: skill %q não encontrada\n", args[0])
		return 1
	}
	if err := skillSvc.Remove(sk, agents); err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(out, "skill %q removida\n", args[0])
	return 0
}

func cmdAdopt(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) int {
	fs := flag.NewFlagSet("adopt", flag.ContinueOnError)
	fs.SetOutput(errOut)
	agentID := fs.String("agent", "", "ID do agente de origem (obrigatório)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(errOut, "uso: lazyagents adopt <skill> --agent <id>")
		return 1
	}
	name := fs.Arg(0)
	// flag para de parsear no primeiro arg posicional; re-parseia o resto
	// para aceitar "adopt <skill> --agent id".
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 1
	}
	if *agentID == "" {
		fmt.Fprintln(errOut, "uso: lazyagents adopt <skill> --agent <id>")
		return 1
	}
	ag, ok := findAgent(*agentID, agents)
	if !ok {
		fmt.Fprintf(errOut, "lazyagents: agente %q não encontrado\n", *agentID)
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(name, skills)
	if !found {
		fmt.Fprintf(errOut, "lazyagents: skill %q não encontrada\n", name)
		return 1
	}
	if err := skillSvc.Adopt(sk, ag); err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	fmt.Fprintf(out, "skill %q adotada na biblioteca\n", name)
	return 0
}

func cmdMigrateLibrary(args []string, c Context, skillSvc *skill.Service) int {
	out, errOut, agents := c.Out, c.Err, c.Agents()
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: lazyagents migrate-library <dir>")
		return 1
	}
	newDir := c.Paths.ExpandHome(args[0])
	fmt.Fprintf(out, "migrando biblioteca para %s...\n", newDir)
	if err := skillSvc.MigrateLibrary(newDir, agents); err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	fmt.Fprintln(out, "migração concluída")
	return 0
}

// SkillChecks são as seções do doctor da feature skills.
func SkillChecks(svc *skill.Service) []Check {
	return []Check{
		{Title: "skills", Run: func(c Context, out io.Writer) []string {
			skills, err := svc.Scan(c.Agents())
			if err != nil {
				fmt.Fprintln(c.Err, "lazyagents:", err)
				return []string{err.Error()}
			}
			var problems []string
			for _, sk := range skills {
				if !sk.Valid {
					problems = append(problems, fmt.Sprintf("skill %q: %s", sk.Dir, sk.Warning))
					fmt.Fprintf(out, "  %-30s AVISO: %s\n", sk.Dir, sk.Warning)
					continue
				}
				issues := skill.Validate(sk)
				if len(issues) == 0 {
					fmt.Fprintf(out, "  %-30s OK\n", sk.Dir)
					continue
				}
				fmt.Fprintf(out, "  %-30s AVISO:\n", sk.Dir)
				for _, iss := range issues {
					problems = append(problems, fmt.Sprintf("skill %q: %s: %s", sk.Dir, iss.Field, iss.Msg))
					fmt.Fprintf(out, "    - %s: %s\n", iss.Field, iss.Msg)
				}
			}
			return problems
		}},
		{Title: "symlinks", Run: func(c Context, out io.Writer) []string {
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
							msg := fmt.Sprintf("symlink quebrado: %s", path)
							problems = append(problems, msg)
							fmt.Fprintf(out, "  AVISO: %s\n", msg)
						}
					}
				}
			}
			if len(problems) == 0 {
				fmt.Fprintln(out, "  tudo OK")
			}
			return problems
		}},
	}
}
