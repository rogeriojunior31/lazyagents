package app

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
	"github.com/rogeriojunior31/lazyagents/internal/feature"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

type diagnosticAdapter struct {
	agent.Adapter
	detected agent.Agent
}

func (a diagnosticAdapter) ID() string          { return a.detected.ID }
func (a diagnosticAdapter) Detect() agent.Agent { return a.detected }

func TestDoctorReportsRealLocalFailuresInBothFrontends(t *testing.T) {
	p := core.PathsIn(t.TempDir())
	for _, dir := range []string{p.ConfigDir, p.HooksDir(), filepath.Join(p.Home, "skills")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	secret := "FIXTURE_SECRET_MUST_NOT_APPEAR"
	config := []byte("future: [" + secret)
	if err := os.WriteFile(p.ConfigPath(), config, 0o600); err != nil {
		t.Fatal(err)
	}
	hook := hooks.Hook{Name: "missing", Hooks: []agent.Hook{{Event: agent.HookSessionStart, Command: "__lazyagents_missing_fixture_executable__"}}}
	svc := hooks.New(nil, p)
	if err := svc.Save(hook); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(p.Home, "skills", "broken")
	if err := os.Symlink(filepath.Join(p.Home, "absent"), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	ad := diagnosticAdapter{detected: agent.Agent{ID: "fixture", Installed: true, ReadDirs: []string{filepath.Dir(link), filepath.Dir(link)}}}
	app := &App{Deps: &feature.Deps{Paths: p, Adapters: []agent.Adapter{ad}}}
	app.features = features(app.doctorReport)
	app.reserveNames()
	defer app.Close()
	report := app.doctorReport(context.Background(), false)
	if report.OK || report.Scope != "local" {
		t.Fatalf("failures missed: %+v", report)
	}
	found := map[string]bool{}
	for _, s := range report.Sections {
		if s.Title == "symlinks" && len(s.Findings) != 1 {
			t.Fatalf("shared link reported more than once: %+v", s.Findings)
		}
		if s.External {
			if !s.Skipped {
				t.Fatal("external check ran implicitly")
			}
			continue
		}
		for _, problem := range s.Findings {
			if problem.Severity == "" || problem.Resource == "" || problem.NextStep == "" {
				t.Fatalf("incomplete finding: %+v", problem)
			}
			found[s.Title] = true
		}
	}
	for _, title := range []string{"configuration", "symlinks", "hooks"} {
		if !found[title] {
			t.Fatalf("missing %s: %+v", title, report)
		}
	}
	var out bytes.Buffer
	for _, cmd := range app.Commands() {
		if cmd.Name == "doctor" {
			if code := cmd.Run(cli.Context{Paths: p, Agents: app.Deps.Agents, Out: &out, Err: &out}, []string{"--local", "--json"}); code != 1 {
				t.Fatal(code)
			}
		}
	}
	var decoded cli.DoctorReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report, decoded) || strings.Contains(out.String(), secret) {
		t.Fatal("CLI differs from shared report or leaked config input")
	}
	for _, tab := range app.Modules() {
		if tab.ID() == "agents" {
			tab.Update(tea.WindowSizeMsg{Width: 36, Height: 13})
			for _, cmd := range tab.(module.Commander).Commands() {
				if cmd.Name == "doctor" {
					run := tab.Update(cmd.Msg)
					tab.Update(run())
					if !strings.Contains(ansi.Strip(tab.View()), "Doctor") || strings.Contains(tab.View(), secret) {
						t.Fatal("TUI report missing or leaked config input")
					}
				}
			}
		}
	}
	after, err := os.ReadFile(p.ConfigPath())
	if err != nil || !bytes.Equal(after, config) {
		t.Fatal("doctor modified invalid configuration")
	}
	if _, err := os.Readlink(link); err != nil {
		t.Fatal("doctor modified broken link")
	}
}
