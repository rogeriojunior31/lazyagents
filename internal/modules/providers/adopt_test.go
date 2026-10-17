package providers

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func writeProviderFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptPreviewApplyClearExternalProviders(t *testing.T) {
	for _, id := range []string{"claude-code", "codex", "pi", "crush"} {
		t.Run(id, func(t *testing.T) {
			paths := core.PathsIn(t.TempDir())
			adapters := agent.All(paths.Home)
			ad := agent.ByID(adapters, id)
			host := ad.(agent.ProviderHost)
			fixtures := map[string]string{}
			switch id {
			case "claude-code":
				fixtures[host.ProviderFile()] = `{"env":{"ANTHROPIC_BASE_URL":"https://external.test/v1","ANTHROPIC_MODEL":"native-model","ANTHROPIC_AUTH_TOKEN":"native-secret","KEEP":"foreign-value"},"unknown":{"x":true}}`
			case "codex":
				fixtures[host.ProviderFile()] = "model_provider = \"external\"\nmodel = \"native-model\"\n[model_providers.external]\nbase_url = \"https://external.test/v1\"\nwire_api = \"responses\"\nenv_key = \"EXTERNAL_KEY\"\n[projects.\"/foreign\"]\ntrust_level = \"trusted\"\n[hooks.state.example]\ntrusted_hash = \"foreign-value\"\n"
			case "pi":
				fixtures[host.ProviderFile()] = `{"providers":{"external":{"baseUrl":"https://external.test/v1","api":"openai-responses","apiKey":"$EXTERNAL_KEY","models":[{"id":"native-model"}],"extra":"foreign-value"}},"unknown":true}`
				fixtures[filepath.Join(filepath.Dir(host.ProviderFile()), "settings.json")] = `{"defaultProvider":"external","defaultModel":"native-model","unknown":"foreign-value"}`
			case "crush":
				fixtures[filepath.Join(filepath.Dir(host.ProviderFile()), "crush.json")] = `{"providers":{"external":{"base_url":"https://external.test/v1","type":"openai-compat","api_key":"${EXTERNAL_KEY}","extra":"foreign-value"}},"models":{"large":{"provider":"external","model":"native-model"}},"unknown":true}`
			}
			for path, content := range fixtures {
				writeProviderFixture(t, path, content)
			}
			svc := New([]agent.Adapter{ad}, paths)
			svc.Detect = func() []agent.Agent { return []agent.Agent{{ID: id, Name: id, Installed: true}} }
			adoption, err := svc.AdoptDraft(id)
			if err != nil {
				t.Fatal(err)
			}
			if adoption.Profile.Token != "" || adoption.Profile.HasToken || adoption.NeedsCredential != (id == "claude-code") {
				t.Fatalf("unsafe adoption: %+v", adoption)
			}
			p := adoption.Profile
			if id == "claude-code" {
				if err := svc.SaveAdoption(adoption, p); err == nil {
					t.Fatal("native token silently adopted")
				}
				p.Token = "reentered-secret"
			}
			if err := svc.SaveAdoption(adoption, p); err != nil {
				t.Fatal(err)
			}
			if err := svc.SaveAdoption(adoption, p); err == nil {
				t.Fatal("adoption replaced a library entry")
			}
			for path, content := range fixtures {
				got, _ := os.ReadFile(path)
				if string(got) != content {
					t.Fatal("import changed native config")
				}
			}
			// Applying a changed draft demonstrates that saving alone did not change live state.
			p.Model = "replacement-model"
			if err := svc.Save(p); err != nil {
				t.Fatal(err)
			}
			st := svc.Status()[0]
			if !st.SavedChanged || st.Applied.Model != "native-model" {
				t.Fatalf("saved/live not separated: %+v", st)
			}
			plan := svc.PrepareSaved(p.Name, []string{id})
			if !plan.valid() || len(plan.Targets[0].Preview.Files) == 0 {
				t.Fatalf("missing preview: %+v", plan)
			}
			encoded, _ := json.Marshal(plan)
			for _, secret := range []string{"native-secret", "reentered-secret", "foreign-value", "Revision", plan.Targets[0].Preview.Revision} {
				if secret != "" && bytes.Contains(encoded, []byte(secret)) {
					t.Fatal("preview exposed a native value or private revision")
				}
			}
			if _, err := os.Stat(paths.BackupsDir()); !os.IsNotExist(err) {
				t.Fatal("preview wrote a backup")
			}
			if r := svc.ApplyReviewed(p.Name, []string{id}, plan); r.AsError() != nil {
				t.Fatal(r.AsError())
			}
			live, active, err := host.ReadProvider()
			if err != nil || !active || live.Model != p.Model {
				t.Fatalf("apply readback: %+v %v", live, err)
			}
			clear := svc.PrepareClear([]string{id})
			if !clear.valid() {
				t.Fatalf("clear preview: %+v", clear)
			}
			if r := svc.ClearReviewed([]string{id}, clear); r.AsError() != nil {
				t.Fatal(r.AsError())
			}
			live, active, err = host.ReadProvider()
			if err != nil || !active || live.BaseURL != p.BaseURL || live.Model != "native-model" {
				t.Fatalf("previous provider lost: %+v %v", live, err)
			}
			for path := range fixtures {
				got, _ := os.ReadFile(path)
				if !bytes.Contains(got, []byte("foreign-value")) {
					t.Fatalf("unknown config lost: %s", got)
				}
			}
		})
	}
}

func TestAdoptionFormAndPreviewCLI(t *testing.T) {
	svc, claude, _, paths := validationService(t)
	original := `{"env":{"ANTHROPIC_BASE_URL":"https://external.test","ANTHROPIC_MODEL":"m","ANTHROPIC_AUTH_TOKEN":"native-secret"},"unknown":"foreign-value"}`
	writeProviderFixture(t, claude.ProviderFile(), original)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	cmd := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	run(t, &m, cmd)
	if m.form != nil || m.preparing {
		t.Fatal("cancelled import reopened")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"}))
	if m.form == nil || m.form.adoption == nil || m.form.hasToken {
		t.Fatal("import did not open a safe draft")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.form == nil || !strings.Contains(m.form.err, "re-enter") {
		t.Fatal("missing credential was accepted")
	}
	m.form.inputs[fToken].SetValue("explicit-secret")
	for _, size := range [][2]int{{36, 8}, {40, 14}, {80, 24}, {120, 11}, {220, 32}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] || strings.Contains(view, "explicit-secret") || !strings.Contains(ansi.Strip(view), "esc") {
			t.Fatalf("form at %v: %s", size, view)
		}
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if !m.statuses[0].External {
		t.Fatal("saving an import claimed native ownership")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeySpace}))
	if m.targets == nil || m.targets.defaults {
		t.Fatal("imported external profile toggled clear instead of apply")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	var out, errs bytes.Buffer
	c := cli.Context{Out: &out, Err: &errs, Paths: paths}
	for _, args := range [][]string{{"apply", "claude-code-imported", "--agent", "claude-code", "--preview", "--json"}, {"clear", "--agent", "claude-code", "--preview", "--json"}} {
		out.Reset()
		errs.Reset()
		if code := cmdProvider(args, c, svc); code != 0 {
			t.Fatalf("%v: %s", args, &errs)
		}
		if !json.Valid(out.Bytes()) || strings.Contains(out.String()+errs.String(), "secret") {
			t.Fatal("unsafe preview output")
		}
	}
	got, _ := os.ReadFile(claude.ProviderFile())
	if string(got) != original {
		t.Fatal("save/preview changed native config")
	}
	if _, err := os.Stat(claude.ProviderState); !os.IsNotExist(err) {
		t.Fatal("preview wrote ownership")
	}
	out.Reset()
	errs.Reset()
	if code := cmdProvider([]string{"adopt", "cli-import", "--agent", "claude-code", "--token", "-"}, cli.Context{Out: &out, Err: &errs, In: strings.NewReader("explicit-cli-secret\n")}, svc); code != 0 {
		t.Fatal(errs.String())
	}
	p, err := svc.Profile("cli-import")
	if err != nil || p.Token != "explicit-cli-secret" || strings.Contains(out.String()+errs.String(), p.Token) {
		t.Fatal("CLI did not preserve explicit credential privately")
	}
}

func TestReviewedOperationsRefuseChangedFilesOrProfiles(t *testing.T) {
	for _, change := range []string{"native", "profile", "token", "state", "clear"} {
		t.Run(change, func(t *testing.T) {
			svc, claude, codex, paths := validationService(t)
			p := agent.ProviderProfile{Name: "p", BaseURL: "http://localhost:1234", Model: "first", EnvKey: "TEST_KEY"}
			if err := svc.Save(p); err != nil {
				t.Fatal(err)
			}
			if change == "clear" {
				if err := svc.Apply("p", ""); err != nil {
					t.Fatal(err)
				}
			}
			plan := svc.PrepareSaved("p", nil)
			if change == "clear" {
				plan = svc.PrepareClear(nil)
			}
			before, _ := os.ReadFile(claude.ProviderFile())
			if change == "profile" || change == "token" {
				if change == "token" {
					p.Token = "changed-saved-secret"
				} else {
					p.Model = "second"
				}
				if err := svc.Save(p); err != nil {
					t.Fatal(err)
				}
			} else if change == "state" {
				writeProviderFixture(t, claude.ProviderState, `{"createdFile":true}`)
			} else {
				data, _ := os.ReadFile(codex.ProviderFile())
				writeProviderFixture(t, codex.ProviderFile(), string(data)+"\n# external change\n")
			}
			var result core.OperationResult
			if change == "clear" {
				result = svc.ClearReviewed(nil, plan)
			} else {
				result = svc.ApplyReviewed("p", nil, plan)
			}
			if result.AsError() == nil || !strings.Contains(result.AsError().Error(), "since review") {
				t.Fatalf("accepted stale review: %+v", result)
			}
			after, _ := os.ReadFile(claude.ProviderFile())
			if !bytes.Equal(before, after) {
				t.Fatal("stale operation partially wrote")
			}
			if change != "clear" {
				if _, err := os.Stat(paths.BackupsDir()); !os.IsNotExist(err) {
					t.Fatal("stale review wrote backup")
				}
			}
		})
	}
}

func TestAdoptionRefusesUnsupportedNativeSources(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	crush, pi, claude := agent.NewCrush(paths.Home), agent.NewPi(paths.Home), agent.NewClaude(paths.Home)
	svc := New([]agent.Adapter{crush, pi, claude}, paths)
	// Executable configuration must never run just to discover a provider.
	marker := filepath.Join(paths.Home, "executed")
	writeProviderFixture(t, crush.ProviderFile(), "touch '"+marker+"'\n")
	writeProviderFixture(t, filepath.Join(filepath.Dir(crush.ProviderFile()), "crush.json"), `{"providers":{"external":{"base_url":"https://external.test","type":"openai-compat"}},"models":{"large":{"provider":"external","model":"m"}}}`)
	if _, err := svc.AdoptDraft("crush"); err == nil {
		t.Fatal("executable rc was ignored during import")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("native script was executed")
	}
	writeProviderFixture(t, pi.ProviderFile(), `{"providers":{"external":{"baseUrl":"https://external.test","api":"unsupported-protocol","models":[{"id":"m"}]}}}`)
	writeProviderFixture(t, filepath.Join(filepath.Dir(pi.ProviderFile()), "settings.json"), `{"defaultProvider":"external","defaultModel":"m"}`)
	if _, err := svc.AdoptDraft("pi"); err == nil {
		t.Fatal("unsupported protocol silently changed")
	}
	writeProviderFixture(t, pi.ProviderFile(), `{"providers":{"external":{"baseUrl":"https://provider.test","api":"openai-completions","models":[{"id":"m","api":"anthropic-messages","baseUrl":"https://model.test"}]}}}`)
	adopted, err := svc.AdoptDraft("pi")
	if err != nil || adopted.Profile.WireAPI != "anthropic" || adopted.Profile.BaseURL != "https://model.test" {
		t.Fatalf("selected model overrides were lost: %+v %v", adopted, err)
	}
	writeProviderFixture(t, pi.ProviderFile(), `{"providers":{"external":{"baseUrl":"https://provider.test","models":[{"id":"m"}]}}}`)
	if _, err := svc.AdoptDraft("pi"); err == nil {
		t.Fatal("missing native protocol was inferred")
	}
	if _, err := svc.AdoptDraft("claude-code"); err == nil {
		t.Fatal("native/default account imported as a provider")
	}
	if _, err := os.Stat(svc.Path()); !os.IsNotExist(err) {
		t.Fatal("failed adoption wrote library")
	}
}

func TestChangedSavedModelSpaceReappliesAndPreviewsStayResponsive(t *testing.T) {
	svc, claude, _, _ := validationService(t)
	p := agent.ProviderProfile{Name: "p", BaseURL: "https://example.test", Model: "old", Token: "secret"}
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply("p", "claude-code"); err != nil {
		t.Fatal(err)
	}
	p.Model = "new"
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	if view := ansi.Strip(m.View()); !strings.Contains(view, "saved settings differ") || !strings.Contains(view, "old") || !strings.Contains(view, "new") {
		t.Fatal(view)
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeySpace}))
	if m.targets == nil || m.targets.defaults {
		t.Fatal("changed saved profile was treated as clear")
	}
	for _, size := range [][2]int{{36, 8}, {40, 14}, {80, 24}, {120, 11}, {220, 32}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] || !strings.Contains(ansi.Strip(view), "esc") || strings.Contains(view, "secret") {
			t.Fatalf("preview at %v: %s", size, view)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil || !strings.Contains(m.confirm.Question, "ANTHROPIC_MODEL") || !strings.Contains(m.confirm.Question, "update:") {
		t.Fatal("confirmation omitted field edits")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	live, _, _ := claude.ReadProvider()
	if live.Model != "new" || m.statuses[0].SavedChanged {
		t.Fatal("changed saved profile was not applied")
	}
}

func TestAdoptingIdenticalClaudePublicFieldsStillTracksApplyOwnership(t *testing.T) {
	svc, claude, _, _ := validationService(t)
	writeProviderFixture(t, claude.ProviderFile(), `{"env":{"ANTHROPIC_BASE_URL":"https://external.test","ANTHROPIC_MODEL":"m"}}`)
	adoption, err := svc.AdoptDraft("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveAdoption(adoption, adoption.Profile); err != nil {
		t.Fatal(err)
	}
	if !svc.Status()[0].External {
		t.Fatal("import acquired ownership")
	}
	if err := svc.Apply(adoption.Profile.Name, "claude-code"); err != nil {
		t.Fatal(err)
	}
	if svc.Status()[0].External {
		t.Fatal("applying identical fields lost ownership")
	}
	plan := svc.PrepareClear([]string{"claude-code"})
	if len(plan.Targets[0].Preview.Files[0].Changes) != 0 {
		t.Fatal("clearing identical originals invented field changes")
	}
	if r := svc.ClearReviewed([]string{"claude-code"}, plan); r.AsError() != nil {
		t.Fatal(r.AsError())
	}
	if !svc.Status()[0].External {
		t.Fatal("clear did not release ownership")
	}
}
