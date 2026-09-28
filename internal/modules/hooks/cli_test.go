package hooks

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

// The hook table and the agent table have different columns: each one aligns
// on its own, so a long hook name does not widen the agent column.
func TestHooksListTablesAlignSeparately(t *testing.T) {
	svc, _ := testService(t)
	h := Hook{Name: "a-rather-long-hook-name", Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "true"}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	c := cli.Context{Out: &out, Err: &errOut}
	if code := cli.Run([]string{"hooks", "list"}, c, commands(svc)); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var agentHeader string
	for line := range strings.Lines(out.String()) {
		if strings.HasPrefix(line, "AGENT") {
			agentHeader = line
		}
	}
	if i := strings.Index(agentHeader, "LAZYAGENTS"); i < 0 || i >= len(h.Name) {
		t.Errorf("agent table padded by the hook table:\n%s", out.String())
	}
}
