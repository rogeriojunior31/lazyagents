package providers

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// provider list --json uses snake_case like every other command, and never
// carries the token without --reveal.
func TestProviderListJSON(t *testing.T) {
	host := &fakeHost{id: "claude-code", installed: true}
	svc := New([]agent.Adapter{host}, core.PathsIn(t.TempDir()))
	p := agent.ProviderProfile{Name: "work", BaseURL: "https://p.example", Token: "sk-secret", EnvKey: "KEY"}
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply("work", "claude-code"); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := cli.Run([]string{"provider", "list", "--json"}, cli.Context{Out: &out, Err: &errOut}, commands(svc)); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	got := out.String()
	for _, want := range []string{`"base_url": "https://p.example"`, `"has_token": true`, `"env_key": "KEY"`, `"applied": {`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	for _, bad := range []string{"sk-secret", "baseUrl", "hasToken"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q must not appear in:\n%s", bad, got)
		}
	}
}

func TestProviderCommandsRejectExtraArgumentsBeforeSideEffects(t *testing.T) {
	svc, _, _, paths := validationService(t)
	p := agent.ProviderProfile{Name: "work", BaseURL: "https://example.test", Model: "m", EnvKey: "TEST_KEY"}
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(p.Name, "claude-code"); err != nil {
		t.Fatal(err)
	}
	before := providerTree(t, paths.Home)
	var out, errs bytes.Buffer
	c := cli.Context{Out: &out, Err: &errs, Paths: paths}
	for _, args := range [][]string{
		{"apply", "work", "extra", "--agent", "codex"},
		{"apply", "work", "--agent", "codex", "extra"},
		{"clear", "extra", "--agent", "codex"},
		{"clear", "--agent", "codex", "extra"},
		{"add", "work", "extra", "--base-url", "https://replacement.test"},
		{"adopt", "imported", "extra", "--agent", "claude-code"},
		{"validate", "work", "extra", "--agent", "codex"},
		{"probe", "work", "extra"},
		{"models", "work", "extra"},
		{"list", "extra", "--json"},
	} {
		out.Reset()
		errs.Reset()
		if code := cmdProvider(args, c, svc); code != 1 {
			t.Fatalf("extra arguments accepted: %v (exit %d)", args, code)
		}
		if !reflect.DeepEqual(before, providerTree(t, paths.Home)) {
			t.Fatalf("invalid command changed library/native files/state/backups: %v", args)
		}
	}
}
