package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestEnvironmentDiagnosticsReachStatusCLIAndTUI(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	host := agent.NewCodex(paths.Home)
	const key = "LAZYAGENTS_M262_INTEGRATION_KEY"
	t.Setenv(key, "")
	if err := host.ApplyProvider(agent.ProviderProfile{Name: "test", BaseURL: "https://public-endpoint.test", EnvKey: key}, ""); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{host}, paths)
	svc.Detect = func() []agent.Agent { return []agent.Agent{{ID: host.ID(), Name: "Codex", Installed: true}} }
	before := providerTree(t, paths.Home)
	statuses := svc.Status()
	if len(statuses) != 1 || len(statuses[0].Environment) != 1 {
		t.Fatalf("missing status diagnostic: %+v", statuses)
	}
	for _, args := range [][]string{{"list"}, {"list", "--json"}} {
		var out, errs bytes.Buffer
		if code := cmdProvider(args, cli.Context{Paths: paths, Out: &out, Err: &errs}, svc); code != 0 {
			t.Fatalf("list failed: %d %s", code, errs.String())
		}
		if !strings.Contains(out.String(), key) || !strings.Contains(out.String(), "current process environment") {
			t.Fatalf("missing CLI evidence: %s", out.String())
		}
	}
	report := cli.RunDoctor(context.Background(), cli.Context{}, checks(svc), false)
	if report.OK || len(report.Sections[0].Findings) != 1 {
		t.Fatalf("doctor should report the missing reference: %+v", report)
	}
	finding := report.Sections[0].Findings[0]
	if finding.Variable != key || finding.Code != "missing_credential_environment" || finding.Source == "" || finding.Tab != "providers" || finding.NextStep == "" {
		t.Fatalf("missing structured doctor evidence: %+v", finding)
	}
	m := newTab(svc)
	m.statuses = statuses
	m.width, m.height = 120, 36
	if view := ansi.Strip(m.View()); !strings.Contains(view, "LIVE SETTINGS") || !strings.Contains(view, key) {
		t.Fatalf("empty library hides environment diagnostics: %s", view)
	}
	m.width, m.height = 40, 12 // Space available below the root's 40x16 header.
	for _, profiles := range [][]agent.ProviderProfile{nil, {{Name: "saved", BaseURL: "https://example.test"}}} {
		m.profiles = profiles
		view := ansi.Strip(m.View())
		if !strings.Contains(strings.Join(strings.Fields(view), ""), key) {
			t.Fatalf("compact layout hides the variable: %s", view)
		}
		lines := strings.Split(view, "\n")
		if len(lines) > m.height {
			t.Fatalf("compact layout exceeds its height: %d > %d", len(lines), m.height)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > m.width {
				t.Fatalf("compact layout exceeds its width: %q", line)
			}
		}
	}
	m.statuses = append(m.statuses, Status{AgentID: "other", AgentName: "Other", Installed: true})
	m.detailOff = 100
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.col != 1 || m.detailOff != 0 || strings.Contains(ansi.Strip(m.View()), key) {
		t.Fatal("compact agent selection retained another agent's diagnostics or scroll")
	}
	m.col, m.profiles, m.statuses = 0, nil, statuses
	detail := ansi.Strip(m.detailContent(40))
	if !strings.Contains(strings.ReplaceAll(detail, "\n", ""), key) || !strings.Contains(detail, "Next:") || !strings.Contains(detail, "Source:") {
		t.Fatalf("missing provider detail: %s", detail)
	}
	_, label := agentState(statuses[0], agent.ProviderProfile{}, false)
	if !strings.Contains(ansi.Strip(label), "environment needs review") {
		t.Fatalf("environment warning is hidden: %s", label)
	}
	if got := providerTree(t, paths.Home); !reflect.DeepEqual(before, got) {
		t.Fatal("read-only diagnostics changed native files or backups")
	}
	t.Setenv(key, "sentinel-token-must-never-appear")
	if got := svc.Status(); len(got[0].Environment) != 0 {
		t.Fatalf("refresh retained stale warning: %+v", got)
	}
	var out, errs bytes.Buffer
	cmdProvider([]string{"list", "--json"}, cli.Context{Paths: paths, Out: &out, Err: &errs}, svc)
	if strings.Contains(out.String()+errs.String(), "sentinel-token") {
		t.Fatal("environment credential was serialized")
	}
}

func TestClaudeDoctorConflictDoesNotPrintComparedValues(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	host := agent.NewClaude(paths.Home)
	t.Setenv("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST", "")
	t.Setenv("ANTHROPIC_BASE_URL", "https://process-private.test")
	t.Setenv("ANTHROPIC_MODEL", "")
	if err := host.ApplyProvider(agent.ProviderProfile{BaseURL: "https://file-private.test", Token: "native-secret-sentinel"}, ""); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{host}, paths)
	svc.Detect = func() []agent.Agent { return []agent.Agent{{ID: host.ID(), Installed: true}} }
	report := cli.RunDoctor(context.Background(), cli.Context{}, checks(svc), false)
	data, _ := json.Marshal(report)
	if report.OK || !strings.Contains(string(data), "environment_config_conflict") {
		t.Fatalf("doctor conflict missing: %s", data)
	}
	for _, value := range []string{"process-private", "file-private", "native-secret-sentinel"} {
		if strings.Contains(string(data), value) {
			t.Fatalf("doctor exposed compared values: %s", data)
		}
	}
}

func TestEnvironmentMetadataErrorsAreSanitized(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	host := agent.NewPi(paths.Home)
	if err := host.ApplyProvider(agent.ProviderProfile{BaseURL: "https://example.test", Model: "m", EnvKey: "LAZYAGENTS_M262_KEY"}, ""); err != nil {
		t.Fatal(err)
	}
	authPath := filepath.Join(filepath.Dir(host.ProviderFile()), "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"lazyagents":{"type":{"private-token-sentinel":1}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New([]agent.Adapter{host}, paths)
	svc.Detect = func() []agent.Agent { return nil }
	got := svc.Status()
	data, _ := json.Marshal(got)
	if len(got[0].Environment) != 1 || got[0].Environment[0].Code != "environment_check_unavailable" || strings.Contains(string(data), "private-token-sentinel") {
		t.Fatalf("unsafe error or missing unreadable evidence: %s", data)
	}
}
