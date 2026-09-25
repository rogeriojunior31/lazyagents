// Package cli implementa o modo headless do lazyagents. Cada feature
// contribui Commands (e opcionalmente Checks para o doctor); o registro vive em
// internal/app. Este arquivo é o framework: dispatch, ajuda gerada e flags.
package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Context é o que todo comando recebe.
type Context struct {
	In       io.Reader // stdin (pass-through de plugins); nil = sem entrada
	Out, Err io.Writer
	Paths    core.Paths
	Agents   func() []agent.Agent // detecção memoizada (roda --version dos CLIs)
	AgentIDs []string             // ids de todos os adapters, sem detecção; nil = não valida
}

// KnownAgent diz se id é um agente suportado; se não for, explica no stderr
// quais são os válidos. id vazio (sem filtro) é sempre aceito.
func (c Context) KnownAgent(id string) bool {
	if id == "" || c.AgentIDs == nil || slices.Contains(c.AgentIDs, id) {
		return true
	}
	fmt.Fprintf(c.Err, "lazyagents: unknown agent %q (valid: %s)\n", id, strings.Join(c.AgentIDs, ", "))
	return false
}

// Command é um subcomando. Run parseia os próprios flags (Flags) e devolve o
// exit code.
type Command struct {
	Name    string
	Usage   string // uma linha, sem o prefixo "lazyagents "
	Summary string // o que o comando faz, em uma frase curta (lista do help)
	Help    string // detalhe opcional para `help <comando>`
	Run     func(c Context, args []string) int
}

// Run despacha args[0] para o comando de mesmo nome.
func Run(args []string, c Context, cmds []Command) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		if len(args) > 1 {
			return help(c, cmds, args[1])
		}
		fmt.Fprint(c.Err, usage(cmds))
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	if cmd, ok := find(cmds, args[0]); ok {
		return cmd.Run(c, args[1:])
	}
	fmt.Fprintf(c.Err, "lazyagents: unknown command %q (see lazyagents help)\n", args[0])
	return 1
}

func find(cmds []Command, name string) (Command, bool) {
	for _, cmd := range cmds {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return Command{}, false
}

// help mostra o detalhe de um comando.
func help(c Context, cmds []Command, name string) int {
	cmd, ok := find(cmds, name)
	if !ok {
		fmt.Fprintf(c.Err, "lazyagents: unknown command %q (see lazyagents help)\n", name)
		return 1
	}
	fmt.Fprintf(c.Out, "usage: lazyagents %s\n", cmd.Usage)
	if cmd.Summary != "" {
		fmt.Fprintf(c.Out, "\n%s\n", cmd.Summary)
	}
	if cmd.Help != "" {
		fmt.Fprintf(c.Out, "\n%s\n", strings.TrimRight(cmd.Help, "\n"))
	}
	return 0
}

func usage(cmds []Command) string {
	width := 0
	for _, cmd := range cmds {
		width = max(width, len(cmd.Name))
	}
	var b strings.Builder
	b.WriteString("usage: lazyagents <command> [options]\n")
	b.WriteString("       lazyagents help <command>   details and options of a command\n")
	b.WriteString("       lazyagents                  open the TUI\n\ncommands:\n")
	for _, cmd := range cmds {
		desc := cmd.Summary
		if desc == "" {
			desc = strings.TrimSpace(strings.TrimPrefix(cmd.Usage, cmd.Name))
		}
		fmt.Fprintf(&b, "  %-*s  %s\n", width, cmd.Name, desc)
	}
	return b.String()
}

// Flags cria o FlagSet de um comando: erros no errOut, sem abortar o
// processo, e ajuda (-h) em inglês.
func Flags(name string, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintf(errOut, "usage: lazyagents %s [options]\n", name)
		n := 0
		fs.VisitAll(func(*flag.Flag) { n++ })
		if n > 0 {
			fmt.Fprintln(errOut, "\noptions:")
			fs.PrintDefaults()
		}
	}
	return fs
}
