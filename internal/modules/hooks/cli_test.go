package hooks

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
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

func TestHooksRemoveInstalledScripts(t *testing.T) {
	svc, home := testService(t)
	if _, err := Import(core.PathsIn(home), DiscoverIn(writeRepo(t), "")[0], "user/repo"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Enable("security-guidance", "claude-code"); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	c := cli.Context{Out: &out, Err: &errOut}
	if code := cli.Run([]string{"hooks", "rm", "security-guidance"}, c, commands(svc)); code != 1 {
		t.Fatalf("exit %d; want failure while installed", code)
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "lazyagents hooks disable security-guidance --agent claude-code") {
		t.Fatalf("missing actionable error: stdout=%q, stderr=%q", &out, &errOut)
	}
	if _, err := svc.Get("security-guidance"); err != nil {
		t.Fatal("failed CLI removal lost the library entry:", err)
	}
	for _, args := range [][]string{
		{"hooks", "disable", "security-guidance", "--agent", "claude-code"},
		{"hooks", "rm", "security-guidance"},
	} {
		if code := cli.Run(args, c, commands(svc)); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, &errOut)
		}
	}
	if _, err := svc.Get("security-guidance"); err == nil {
		t.Fatal("CLI removal kept the uninstalled entry")
	}
}
