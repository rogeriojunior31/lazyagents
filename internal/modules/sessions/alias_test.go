package sessions

import (
	"os"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestAliasRoundTrip(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	// same id in two agents: aliases are per agent
	a := agent.Session{AgentID: "claude-code", ID: "s1", Title: "t1"}
	b := agent.Session{AgentID: "codex", ID: "s1", Title: "t2"}
	svc := New([]agent.Adapter{
		fakeAdapter{id: "claude-code", sessions: []agent.Session{a}},
		fakeAdapter{id: "codex", sessions: []agent.Session{b}},
	}, paths)

	// an unknown key survives the rewrite
	if err := os.MkdirAll(paths.DataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.AliasesPath(), []byte(`{"version": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetAlias(a, "  refactor auth  "); err != nil {
		t.Fatal(err)
	}
	// "restart": a new service reads from disk
	svc2 := New(svc.adapters, paths)
	got, err := svc2.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range got {
		want := ""
		if s.AgentID == "claude-code" {
			want = "refactor auth"
		}
		if s.Alias != want {
			t.Errorf("%s:%s alias = %q, want %q", s.AgentID, s.ID, s.Alias, want)
		}
	}
	data, _ := os.ReadFile(paths.AliasesPath())
	if !strings.Contains(string(data), `"version": 2`) {
		t.Errorf("unknown key lost:\n%s", data)
	}
	// blank removes
	if err := svc2.SetAlias(a, "   "); err != nil {
		t.Fatal(err)
	}
	got, _ = svc2.List()
	for _, s := range got {
		if s.Alias != "" {
			t.Errorf("alias should be removed: %+v", s)
		}
	}
}

func TestAliasCorruptFileDoesNotHideSessions(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	svc := New([]agent.Adapter{fakeAdapter{id: "x", sessions: []agent.Session{{AgentID: "x", ID: "1"}}}}, paths)
	if err := os.MkdirAll(paths.DataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.AliasesPath(), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := svc.List()
	if len(got) != 1 || err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Errorf("List = %d sessions, err = %v", len(got), err)
	}
	if err := svc.SetAlias(got[0], "x"); err == nil {
		t.Error("SetAlias should not overwrite a corrupt file")
	}
}

func TestNullAliases(t *testing.T) {
	for _, data := range []string{"null", `{"aliases":null}`} {
		t.Run(data, func(t *testing.T) {
			p := core.PathsIn(t.TempDir())
			if err := os.MkdirAll(p.DataDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p.AliasesPath(), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			svc := New(nil, p)
			if err := svc.SetAlias(agent.Session{ID: "s", AgentID: "codex"}, "label"); err != nil {
				t.Fatal(err)
			}
			_, aliases, err := svc.readAliases()
			if err != nil || aliases["codex:s"] != "label" {
				t.Fatalf("%v %v", aliases, err)
			}
		})
	}
}
