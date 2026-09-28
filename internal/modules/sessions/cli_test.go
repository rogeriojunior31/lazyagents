package sessions

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// runSessions dispatches a module command the way internal/app does.
func runSessions(args []string, out *bytes.Buffer, svc *Service) int {
	c := cli.Context{Out: out, Err: out, Agents: func() []agent.Agent { return nil }}
	return cli.Run(args, c, commands(svc))
}

// claudeSessionHome writes one Claude Code session with usage of a priced model.
func claudeSessionHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "-tmp-proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonl := `{"type":"user","message":{"role":"user","content":"hi"},"cwd":"/tmp/x"}
{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"a"}],"usage":{"input_tokens":100,"output_tokens":200,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(filepath.Join(proj, "sess-1.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func sessionsJSON(t *testing.T, home string) []jsonSessionItem {
	t.Helper()
	sessSvc := New([]agent.Adapter{agent.NewClaude(home)}, core.PathsIn(home))
	var out bytes.Buffer
	code := runSessions([]string{"sessions", "--json"}, &out, sessSvc)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var items []jsonSessionItem
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("invalid JSON: %v — output: %q", err, out.String())
	}
	if len(items) != 1 {
		t.Fatalf("want 1 session, got %d", len(items))
	}
	return items
}

func TestCLISessionsJSONUsage(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test") // API-key account: billed per token
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	u := sessionsJSON(t, claudeSessionHome(t))[0].Usage
	if u == nil {
		t.Fatal("want the usage field")
	}
	if u.Input != 100 || u.Output != 200 || u.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("usage: %+v", u)
	}
	if u.CostUSD == nil {
		t.Error("want an estimated cost for a known model")
	}
}

// A subscription is not billed per token: tokens stay, cost_usd goes.
func TestCLISessionsJSONNoCostForSubscription(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	home := claudeSessionHome(t)
	creds := `{"claudeAiOauth":{"subscriptionType":"max","accessToken":"x"}}`
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}
	u := sessionsJSON(t, home)[0].Usage
	if u == nil || u.Input != 100 {
		t.Fatalf("want tokens, got %+v", u)
	}
	if u.CostUSD != nil {
		t.Errorf("cost_usd = %v, want none for a subscription", *u.CostUSD)
	}
}

func TestFilterSessions(t *testing.T) {
	list := []agent.Session{
		{ID: "1", AgentID: "a", CWD: "/p"},
		{ID: "2", AgentID: "b", CWD: "/p/"},
		{ID: "3", AgentID: "a", CWD: "/q"},
		{ID: "4", AgentID: "a", CWD: "/p"},
	}
	ids := func(ss []agent.Session) (out string) {
		for _, s := range ss {
			out += s.ID
		}
		return out
	}
	cases := []struct {
		agentID, cwd string
		limit        int
		want         string
	}{
		{"", "", 0, "1234"},
		{"a", "", 0, "134"},
		{"", "/p", 0, "124"},
		{"a", "/p", 1, "1"},
	}
	for _, tc := range cases {
		if got := ids(filter(list, tc.agentID, tc.cwd, tc.limit)); got != tc.want {
			t.Errorf("filter(%q, %q, %d) = %s, want %s", tc.agentID, tc.cwd, tc.limit, got, tc.want)
		}
	}
}
