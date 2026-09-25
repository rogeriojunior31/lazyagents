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

// cmds is a minimal registry that exercises only the dispatch.
func cmds() []Command {
	return []Command{
		{Name: "echo", Usage: "echo [text]", Run: func(c Context, args []string) int {
			_, _ = c.Out.Write([]byte(strings.Join(args, " ")))
			return 7
		}},
	}
}

func TestRunDispatchAndExitCode(t *testing.T) {
	var out bytes.Buffer
	code := Run([]string{"echo", "hi", "world"}, Context{Out: &out, Err: &out}, cmds())
	if code != 7 || out.String() != "hi world" {
		t.Errorf("Run = %d, output %q", code, out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if code := Run([]string{"xyzzy"}, Context{Out: &out, Err: &out}, cmds()); code != 1 {
		t.Errorf("unknown command = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Errorf("output = %q", out.String())
	}
}

// No arguments: usage on stderr and exit 1; `help` prints the same with exit 0.
func TestRunUsage(t *testing.T) {
	var out bytes.Buffer
	if code := Run(nil, Context{Out: &out, Err: &out}, cmds()); code != 1 {
		t.Errorf("no args = %d, want 1", code)
	}
	var help bytes.Buffer
	if code := Run([]string{"help"}, Context{Out: &help, Err: &help}, cmds()); code != 0 {
		t.Errorf("help = %d, want 0", code)
	}
	if !strings.Contains(help.String(), "echo  [text]") { // no Summary: shows the arguments
		t.Errorf("usage does not list the commands: %q", help.String())
	}
}

func TestHelpCommand(t *testing.T) {
	list := []Command{{Name: "echo", Usage: "echo [text]", Summary: "repeats the text", Help: "long detail"}}
	var out bytes.Buffer
	if code := Run([]string{"help", "echo"}, Context{Out: &out, Err: &out}, list); code != 0 {
		t.Fatalf("help echo = %d", code)
	}
	for _, want := range []string{"usage: lazyagents echo [text]", "repeats the text", "long detail"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help echo without %q: %q", want, out.String())
		}
	}
	out.Reset()
	Run([]string{"help"}, Context{Out: &out, Err: &out}, list)
	if !strings.Contains(out.String(), "echo  repeats the text") {
		t.Errorf("help list without summary: %q", out.String())
	}
	if code := Run([]string{"help", "nothing"}, Context{Out: &out, Err: &out}, list); code != 1 {
		t.Errorf("help of a missing command = %d", code)
	}
}

func TestKnownAgent(t *testing.T) {
	var errOut bytes.Buffer
	c := Context{Err: &errOut, AgentIDs: []string{"a", "b"}}
	if !c.KnownAgent("") || !c.KnownAgent("a") || errOut.Len() != 0 {
		t.Fatal("empty and valid ids must pass silently")
	}
	if c.KnownAgent("z") || !strings.Contains(errOut.String(), "valid: a, b") {
		t.Errorf("invalid id: %q", errOut.String())
	}
}

func TestDoctorJSON(t *testing.T) {
	checks := []Check{{Title: "x", Run: func(c Context, out io.Writer) []string {
		fmt.Fprintln(out, "  ✗ broken")
		return []string{"broken"}
	}}}
	var out bytes.Buffer
	c := Context{Out: &out, Err: &out, Agents: func() []agent.Agent { return []agent.Agent{{ID: "a", Installed: true}} }}
	if code := DoctorCommand(checks).Run(c, []string{"--json"}); code != 1 {
		t.Errorf("doctor --json with a problem = %d", code)
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
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if rep.OK || len(rep.Agents) != 1 || rep.Sections[0].Problems[0] != "broken" || !strings.Contains(rep.Sections[0].Report, "broken") {
		t.Errorf("report = %+v", rep)
	}
}
