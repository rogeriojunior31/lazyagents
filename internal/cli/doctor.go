package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Check é uma seção do doctor contribuída por uma feature: escreve o relatório
// em out e devolve os problemas encontrados (vazio = tudo OK).
type Check struct {
	Title string
	Run   func(c Context, out io.Writer) []string
}

// DoctorCommand agrega as checagens de todas as features. Exit 1 se houver
// qualquer problema.
func DoctorCommand(checks []Check) Command {
	return Command{
		Name:    "doctor",
		Usage:   "doctor [--json]",
		Summary: "diagnostica agentes, skills, hooks, provedores, uso e plugins",
		Run: func(c Context, args []string) int {
			fs := Flags("doctor", c.Err)
			jsonOut := fs.Bool("json", false, "saída JSON")
			if err := fs.Parse(args); err != nil {
				return 1
			}
			if *jsonOut {
				return doctorJSON(c, checks)
			}
			fmt.Fprintln(c.Out, "=== agentes detectados ===")
			for _, ag := range c.Agents() {
				status := "não instalado"
				if ag.Installed {
					status = "instalado"
				}
				fmt.Fprintf(c.Out, "  %-20s %s\n", ag.ID, status)
			}
			var problems []string
			for _, ch := range checks {
				fmt.Fprintf(c.Out, "\n=== %s ===\n", ch.Title)
				problems = append(problems, ch.Run(c, c.Out)...)
			}
			if len(problems) > 0 {
				fmt.Fprintf(c.Out, "\n%d problema(s) encontrado(s)\n", len(problems))
				return 1
			}
			fmt.Fprintln(c.Out, "\ntudo OK")
			return 0
		},
	}
}

// doctorJSON é a forma estável do `doctor --json`: o relatório de cada seção
// vai como texto (é o que as checagens produzem) e os problemas como lista.
func doctorJSON(c Context, checks []Check) int {
	type agentItem struct {
		ID        string `json:"id"`
		Installed bool   `json:"installed"`
		Version   string `json:"version,omitempty"`
	}
	type section struct {
		Title    string   `json:"title"`
		Report   string   `json:"report"`
		Problems []string `json:"problems"`
	}
	report := struct {
		OK       bool        `json:"ok"`
		Agents   []agentItem `json:"agents"`
		Sections []section   `json:"sections"`
	}{OK: true, Agents: []agentItem{}, Sections: []section{}}
	for _, ag := range c.Agents() {
		report.Agents = append(report.Agents, agentItem{ag.ID, ag.Installed, ag.Version})
	}
	for _, ch := range checks {
		var buf bytes.Buffer
		problems := ch.Run(c, &buf)
		if problems == nil {
			problems = []string{}
		}
		report.OK = report.OK && len(problems) == 0
		report.Sections = append(report.Sections, section{ch.Title, strings.TrimRight(buf.String(), "\n"), problems})
	}
	enc := json.NewEncoder(c.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
	if !report.OK {
		return 1
	}
	return 0
}
