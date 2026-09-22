package cli

import (
	"bytes"
	"strings"
	"testing"
)

// cmds é um registro mínimo para exercitar só o dispatch.
func cmds() []Command {
	return []Command{
		{Name: "eco", Usage: "eco [texto]", Run: func(c Context, args []string) int {
			_, _ = c.Out.Write([]byte(strings.Join(args, " ")))
			return 7
		}},
	}
}

func TestRunDispatchAndExitCode(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"eco", "oi", "mundo"}, Context{Out: &out, Err: &out}, cmds())
	if code != 7 || out.String() != "oi mundo" {
		t.Errorf("Run = %d, saída %q", code, out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if code := Run([]string{"xyzzy"}, Context{Out: &out, Err: &out}, cmds()); code != 1 {
		t.Errorf("comando desconhecido = %d, queria 1", code)
	}
	if !strings.Contains(out.String(), "desconhecido") {
		t.Errorf("saída = %q", out.String())
	}
}

// Sem argumentos: usage no stderr e exit 1; `help` mostra o mesmo com exit 0.
func TestRunUsage(t *testing.T) {
	var out bytes.Buffer
	if code := Run(nil, Context{Out: &out, Err: &out}, cmds()); code != 1 {
		t.Errorf("sem args = %d, queria 1", code)
	}
	var help bytes.Buffer
	if code := Run([]string{"help"}, Context{Out: &help, Err: &help}, cmds()); code != 0 {
		t.Errorf("help = %d, queria 0", code)
	}
	if !strings.Contains(help.String(), "eco [texto]") {
		t.Errorf("usage não lista os comandos: %q", help.String())
	}
}
