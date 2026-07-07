// Package cli implementa o modo headless do lazyskills.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"lazyskills/internal/agent"
	"lazyskills/internal/session"
	"lazyskills/internal/skill"
)

// Run executa o subcomando e devolve o exit code.
func Run(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent, sessionSvc *session.Service) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: lazyskills <comando> [opções]\n\ncomandos: list, enable, disable, install, remove, adopt, sessions, doctor, migrate-library")
		return 1
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "list":
		return cmdList(rest, out, errOut, skillSvc, agents)
	case "enable":
		return cmdToggle(rest, out, errOut, skillSvc, agents, true)
	case "disable":
		return cmdToggle(rest, out, errOut, skillSvc, agents, false)
	case "install":
		return cmdInstall(rest, out, errOut, skillSvc)
	case "remove":
		return cmdRemove(rest, out, errOut, skillSvc, agents)
	case "adopt":
		return cmdAdopt(rest, out, errOut, skillSvc, agents)
	case "sessions":
		return cmdSessions(rest, out, errOut, sessionSvc)
	case "doctor":
		return cmdDoctor(out, errOut, skillSvc, agents)
	case "migrate-library":
		return cmdMigrateLibrary(rest, out, errOut, skillSvc, agents)
	default:
		fmt.Fprintf(errOut, "lazyskills: comando desconhecido %q\n", cmd)
		return 1
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
	ID      string `json:"id"`
	Agent   string `json:"agent"`
	Title   string `json:"title"`
	CWD     string `json:"cwd,omitempty"`
	Updated string `json:"updated,omitempty"`
}

// --- helpers ---

func withSkills(errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) ([]skill.Skill, bool) {
	skills, err := skillSvc.Scan(agents)
	if err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
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

func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return strings.Replace(path, home, "~", 1)
}

// --- comandos ---

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
			fmt.Fprintln(errOut, "lazyskills:", err)
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
		fmt.Fprintf(errOut, "uso: lazyskills %s <skill> [--agent id|--all]\n", verb)
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
		fmt.Fprintf(errOut, "lazyskills: skill %q não encontrada\n", name)
		return 1
	}

	targets := agents
	if *agentID != "" {
		ag, ok := findAgent(*agentID, agents)
		if !ok {
			fmt.Fprintf(errOut, "lazyskills: agente %q não encontrado\n", *agentID)
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
			fmt.Fprintln(errOut, "lazyskills:", e)
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
		fmt.Fprintln(errOut, "uso: lazyskills install <origem>")
		return 1
	}
	found, origin, cleanup, err := skillSvc.Discover(args[0])
	if err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
		return 1
	}
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	names, err := skillSvc.Install(found, origin)
	if err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
		return 1
	}
	for _, n := range names {
		fmt.Fprintf(out, "instalada: %s\n", n)
	}
	return 0
}

func cmdRemove(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: lazyskills remove <skill>")
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(args[0], skills)
	if !found {
		fmt.Fprintf(errOut, "lazyskills: skill %q não encontrada\n", args[0])
		return 1
	}
	if err := skillSvc.Remove(sk, agents); err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
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
		fmt.Fprintln(errOut, "uso: lazyskills adopt <skill> --agent <id>")
		return 1
	}
	name := fs.Arg(0)
	// flag para de parsear no primeiro arg posicional; re-parseia o resto
	// para aceitar "adopt <skill> --agent id".
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 1
	}
	if *agentID == "" {
		fmt.Fprintln(errOut, "uso: lazyskills adopt <skill> --agent <id>")
		return 1
	}
	ag, ok := findAgent(*agentID, agents)
	if !ok {
		fmt.Fprintf(errOut, "lazyskills: agente %q não encontrado\n", *agentID)
		return 1
	}
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}
	sk, found := findSkill(name, skills)
	if !found {
		fmt.Fprintf(errOut, "lazyskills: skill %q não encontrada\n", name)
		return 1
	}
	if err := skillSvc.Adopt(sk, ag); err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
		return 1
	}
	fmt.Fprintf(out, "skill %q adotada na biblioteca\n", name)
	return 0
}

func cmdSessions(args []string, out, errOut io.Writer, sessionSvc *session.Service) int {
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonOut := fs.Bool("json", false, "saída JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if sessionSvc == nil {
		fmt.Fprintln(errOut, "lazyskills: service de sessões não disponível")
		return 1
	}
	sessions, err := sessionSvc.List()
	if err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
		return 1
	}
	if *jsonOut {
		items := make([]jsonSessionItem, 0, len(sessions))
		for _, s := range sessions {
			items = append(items, jsonSessionItem{
				ID:      s.ID,
				Agent:   s.AgentID,
				Title:   s.Title,
				CWD:     tilde(s.CWD),
				Updated: s.MTime.Format("2006-01-02 15:04"),
			})
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(items)
		return 0
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENTE\tTÍTULO\tCWD\tATUALIZADO")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.AgentID, s.Title, tilde(s.CWD), s.MTime.Format("2006-01-02 15:04"))
	}
	_ = tw.Flush()
	return 0
}

func cmdDoctor(out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) int {
	skills, ok := withSkills(errOut, skillSvc, agents)
	if !ok {
		return 1
	}

	var problems []string

	fmt.Fprintln(out, "=== agentes detectados ===")
	for _, ag := range agents {
		status := "não instalado"
		if ag.Installed {
			status = "instalado"
		}
		fmt.Fprintf(out, "  %-20s %s\n", ag.ID, status)
	}

	fmt.Fprintln(out, "\n=== skills ===")
	for _, sk := range skills {
		if !sk.Valid {
			problems = append(problems, fmt.Sprintf("skill %q: %s", sk.Dir, sk.Warning))
			fmt.Fprintf(out, "  %-30s AVISO: %s\n", sk.Dir, sk.Warning)
			continue
		}
		fmt.Fprintf(out, "  %-30s OK\n", sk.Dir)
	}

	fmt.Fprintln(out, "\n=== symlinks ===")
	broken := 0
	for _, ag := range agents {
		for _, dir := range ag.ReadDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				path := filepath.Join(dir, e.Name())
				info, err := os.Lstat(path)
				if err != nil {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					if _, err := os.Stat(path); os.IsNotExist(err) {
						msg := fmt.Sprintf("symlink quebrado: %s", path)
						problems = append(problems, msg)
						fmt.Fprintf(out, "  AVISO: %s\n", msg)
						broken++
					}
				}
			}
		}
	}
	if broken == 0 {
		fmt.Fprintln(out, "  tudo OK")
	}

	if len(problems) > 0 {
		fmt.Fprintf(out, "\n%d problema(s) encontrado(s)\n", len(problems))
		return 1
	}
	fmt.Fprintln(out, "\ntudo OK")
	return 0
}

func cmdMigrateLibrary(args []string, out, errOut io.Writer, skillSvc *skill.Service, agents []agent.Agent) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: lazyskills migrate-library <dir>")
		return 1
	}
	newDir := args[0]
	if len(newDir) >= 2 && newDir[:2] == "~/" {
		home, _ := os.UserHomeDir()
		newDir = filepath.Join(home, newDir[2:])
	}
	fmt.Fprintf(out, "migrando biblioteca para %s...\n", newDir)
	if err := skillSvc.MigrateLibrary(newDir, agents); err != nil {
		fmt.Fprintln(errOut, "lazyskills:", err)
		return 1
	}
	fmt.Fprintln(out, "migração concluída")
	return 0
}
