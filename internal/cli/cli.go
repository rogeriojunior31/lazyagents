// Package cli implementa o modo headless do lazyagents. Cada feature
// contribui Commands (e opcionalmente Checks para o doctor); o registro vive em
// internal/app. Este arquivo é o framework: dispatch e usage gerado.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Context é o que todo comando recebe.
type Context struct {
	Out, Err io.Writer
	Paths    core.Paths
	Agents   func() []agent.Agent // detecção memoizada (roda --version dos CLIs)
}

// Command é um subcomando. Run parseia os próprios flags (stdlib flag) e
// devolve o exit code.
type Command struct {
	Name  string
	Usage string // uma linha, sem o prefixo "lazyagents "
	Run   func(c Context, args []string) int
}

// Run despacha args[0] para o comando de mesmo nome.
func Run(args []string, c Context, cmds []Command) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(c.Err, usage(cmds))
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	for _, cmd := range cmds {
		if cmd.Name == args[0] {
			return cmd.Run(c, args[1:])
		}
	}
	fmt.Fprintf(c.Err, "lazyagents: comando desconhecido %q\n", args[0])
	return 1
}

func usage(cmds []Command) string {
	var b strings.Builder
	b.WriteString("uso: lazyagents <comando> [opções]\n\ncomandos:\n")
	for _, cmd := range cmds {
		fmt.Fprintf(&b, "  %s\n", cmd.Usage)
	}
	return b.String()
}
