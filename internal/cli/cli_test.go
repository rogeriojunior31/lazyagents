package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
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
	if !strings.Contains(help.String(), "eco  [texto]") { // sem Summary, mostra os argumentos
		t.Errorf("usage não lista os comandos: %q", help.String())
	}
}

func TestHelpCommand(t *testing.T) {
	list := []Command{{Name: "eco", Usage: "eco [texto]", Summary: "repete o texto", Help: "detalhe longo"}}
	var out bytes.Buffer
	if code := Run([]string{"help", "eco"}, Context{Out: &out, Err: &out}, list); code != 0 {
		t.Fatalf("help eco = %d", code)
	}
	for _, want := range []string{"uso: lazyagents eco [texto]", "repete o texto", "detalhe longo"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help eco sem %q: %q", want, out.String())
		}
	}
	out.Reset()
	Run([]string{"help"}, Context{Out: &out, Err: &out}, list)
	if !strings.Contains(out.String(), "eco  repete o texto") {
		t.Errorf("lista do help sem resumo: %q", out.String())
	}
	if code := Run([]string{"help", "nada"}, Context{Out: &out, Err: &out}, list); code != 1 {
		t.Errorf("help de comando inexistente = %d", code)
	}
}

func TestKnownAgent(t *testing.T) {
	var errOut bytes.Buffer
	c := Context{Err: &errOut, AgentIDs: []string{"a", "b"}}
	if !c.KnownAgent("") || !c.KnownAgent("a") || errOut.Len() != 0 {
		t.Fatal("id vazio e id válido devem passar em silêncio")
	}
	if c.KnownAgent("z") || !strings.Contains(errOut.String(), "válidos: a, b") {
		t.Errorf("id inválido: %q", errOut.String())
	}
}

func TestDoctorJSON(t *testing.T) {
	checks := []Check{{Title: "x", Run: func(c Context, out io.Writer) []string {
		fmt.Fprintln(out, "  ✗ quebrado")
		return []string{"quebrado"}
	}}}
	var out bytes.Buffer
	c := Context{Out: &out, Err: &out, Agents: func() []agent.Agent { return []agent.Agent{{ID: "a", Installed: true}} }}
	if code := DoctorCommand(checks).Run(c, []string{"--json"}); code != 1 {
		t.Errorf("doctor --json com problema = %d", code)
	}
	var rep struct {
		OK       bool
		Agents   []struct{ ID string }
		Sections []struct {
			Report   string
			Problems []string
		}
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, out.String())
	}
	if rep.OK || len(rep.Agents) != 1 || rep.Sections[0].Problems[0] != "quebrado" || !strings.Contains(rep.Sections[0].Report, "quebrado") {
		t.Errorf("relatório = %+v", rep)
	}
}
