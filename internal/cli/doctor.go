package cli

import (
	"fmt"
	"io"
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
	return Command{Name: "doctor", Usage: "doctor", Run: func(c Context, _ []string) int {
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
	}}
}
