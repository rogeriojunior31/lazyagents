package providers

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/modules/recovery"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestPublicMatchesDistinguishProtocolAndCredentialReferences(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	for _, id := range []string{"claude-code", "codex", "pi", "crush"} {
		t.Run(id, func(t *testing.T) {
			host := agent.ByID(agent.All(paths.Home), id).(agent.ProviderHost)
			p := agent.ProviderProfile{Name: "right", BaseURL: "https://example.test/v1", Model: "m", EnvKey: "KEY_A"}
			live := p.Redacted()
			switch id {
			case "claude-code":
				live.WireAPI = "anthropic"
			case "codex":
				live.WireAPI = "responses"
			default:
				live.WireAPI = "chat"
			}
			wrongProtocol := p
			wrongProtocol.Name = "wrong-protocol"
			wrongProtocol.WireAPI = "anthropic"
			wrongRef := p
			wrongRef.Name = "wrong-ref"
			wrongRef.EnvKey = "KEY_B"
			comparisons := compareProfiles(host, live, true, []agent.ProviderProfile{wrongProtocol, wrongRef, p})
			name, matches := matchProfile(comparisons)
			if id == "claude-code" {
				if name != "" || len(matches) != 3 {
					t.Fatal("ignored Claude selectors were used to identify a profile")
				}
			} else if name != "right" || !reflect.DeepEqual(matches, []string{"right"}) {
				t.Fatalf("false match: %q %v", name, matches)
			}
			alias := p
			alias.Name = "alias"
			alias.WireAPI = live.WireAPI
			name, matches = matchProfile(compareProfiles(host, live, true, []agent.ProviderProfile{p, alias}))
			if name != "" || len(matches) != 2 {
				t.Fatal("equivalent defaults silently selected a profile")
			}
			first, second := p, p
			first.Name = "secret-a"
			second.Name = "secret-b"
			first.Token = "fixture-secret-a"
			second.Token = "fixture-secret-b"
			live = first.Redacted()
			name, matches = matchProfile(compareProfiles(host, live, true, []agent.ProviderProfile{first, second}))
			if name != "" || len(matches) != 2 {
				t.Fatal("secret values were used to distinguish profiles")
			}
		})
	}
}

func TestPublicDriftRefreshRetainAndReapplyAcrossAdapters(t *testing.T) {
	for _, id := range []string{"claude-code", "codex", "pi", "crush"} {
		t.Run(id, func(t *testing.T) {
			paths := core.PathsIn(t.TempDir())
			ad := agent.ByID(agent.All(paths.Home), id)
			host := ad.(agent.ProviderHost)
			svc := New([]agent.Adapter{ad}, paths)
			svc.Detect = func() []agent.Agent { return []agent.Agent{{ID: id, Name: id, Installed: true}} }
			p := agent.ProviderProfile{Name: "work", BaseURL: "https://example.test/v1", Model: "m", EnvKey: "KEY_A"}
			if id == "claude-code" {
				p.Token = "fixture-native-secret"
			}
			if err := svc.Save(p); err != nil {
				t.Fatal(err)
			}
			initial := svc.Status()[0]
			if initial.Active || initial.Profile != "" || len(initial.Comparisons) != 1 || len(initial.Comparisons[0].Differences) == 0 {
				t.Fatal("native defaults were claimed as a saved profile")
			}
			if err := svc.Apply(p.Name, id); err != nil {
				t.Fatal(err)
			}
			if st := svc.Status()[0]; st.Profile != p.Name || len(st.Matches) != 1 || st.SavedChanged {
				t.Fatalf("apply did not match: %+v", st)
			}
			// Simulate a native edit without any service/library write.
			file := host.ProviderFile()
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var edited []byte
			if id == "claude-code" {
				edited = bytes.ReplaceAll(data, []byte("https://example.test/v1"), []byte("https://external.test/v1"))
			} else {
				edited = bytes.ReplaceAll(data, []byte("KEY_A"), []byte("KEY_B"))
			}
			if bytes.Equal(data, edited) {
				t.Fatal("fixture did not edit a native public field")
			}
			if err := os.WriteFile(file, edited, 0600); err != nil {
				t.Fatal(err)
			}
			m := newTab(svc)
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
			run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
			st := m.statuses[0]
			if st.Profile != "" || len(st.Matches) != 0 || !st.SavedChanged {
				t.Fatalf("external edit hidden: %+v", st)
			}
			comparison, ok := st.comparison(p.Name)
			field := "credential"
			if id == "claude-code" {
				field = "endpoint"
			}
			if !ok || len(comparison.Differences) != 1 || comparison.Differences[0].Field != field {
				t.Fatalf("wrong drift fields: %+v", comparison)
			}
			before := providerTree(t, paths.Home)
			run(t, &m, m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}))
			if !reflect.DeepEqual(before, providerTree(t, paths.Home)) {
				t.Fatal("refresh rewrote configuration/state/backups")
			}
			for _, size := range [][2]int{{40, 14}, {80, 24}, {120, 32}, {220, 32}} {
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				view := m.View()
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] || strings.Contains(view, p.Token) && p.Token != "" {
					t.Fatalf("drift view at %v: %s", size, ansi.Strip(view))
				}
			}
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
			if detail := ansi.Strip(m.detailContent(100)); !strings.Contains(detail, field+": saved") || !strings.Contains(detail, "Keep live") || !strings.Contains(detail, "Reapply: p") {
				t.Fatal(detail)
			}
			run(t, &m, m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"}))
			if m.targets == nil || m.targets.defaults {
				t.Fatal("reapply action did not prepare an apply")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if !reflect.DeepEqual(before, providerTree(t, paths.Home)) {
				t.Fatal("keeping live/cancelling preview changed files")
			}
			run(t, &m, m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"}))
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
			if m.report == nil || m.report.Result.AsError() != nil || m.statuses[0].Profile != p.Name {
				t.Fatal("confirmed reapply did not converge")
			}
			// Each adapter's drift snapshot remains recoverable after clearing.
			recoveries := recovery.New([]agent.Adapter{ad}, paths.BackupsDir())
			entries, err := recoveries.List()
			if err != nil {
				t.Fatal(err)
			}
			var snapshot string
			for _, entry := range entries {
				archived, err := os.ReadFile(filepath.Join(paths.BackupsDir(), entry.ID))
				if err == nil && entry.Restorable && entry.Destination == file && bytes.Equal(archived, edited) {
					snapshot = entry.ID
				}
			}
			if snapshot == "" {
				t.Fatal("native drift was not backed up with recoverable provenance")
			}
			clear := svc.PrepareClear([]string{id})
			if result := svc.ClearReviewed([]string{id}, clear); result.AsError() != nil {
				t.Fatal(result.AsError())
			}
			preview, err := recoveries.Preview(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if err := recoveries.Restore(snapshot, preview.Review); err != nil {
				t.Fatal(err)
			}
			if restored, err := os.ReadFile(file); err != nil || !bytes.Equal(restored, edited) {
				t.Fatal("recovery did not restore the reviewed native file")
			}
		})
	}
}

func providerTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			out[path] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAmbiguityAndSecretOnlyEditsRequireExplicitProfileSelection(t *testing.T) {
	svc, claude, _, _ := validationService(t)
	p := agent.ProviderProfile{Name: "first", BaseURL: "https://example.test", Model: "m", Token: "fixture-old-secret"}
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(p.Name, "claude-code"); err != nil {
		t.Fatal(err)
	}
	p.Name = "second"
	p.Token = "fixture-new-secret"
	p.EnvKey = "IGNORED_REF"
	p.WireAPI = "responses"
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IGNORED_REF", "never-read-env-secret")
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	if st := m.statuses[0]; st.Profile != "" || !reflect.DeepEqual(st.Matches, []string{"first", "second"}) {
		t.Fatalf("ambiguous identity inferred: %+v", st)
	}
	var out, errs bytes.Buffer
	if code := providerList([]string{"--json"}, cli.Context{Out: &out, Err: &errs}, svc); code != 0 {
		t.Fatal("list failed")
	}
	var result struct {
		Agents []jsonStatus `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Agents[0].Profile != "" || len(result.Agents[0].Matches) != 2 {
		t.Fatal("JSON hid ambiguity")
	}
	for _, secret := range []string{"fixture-old-secret", "fixture-new-secret", "never-read-env-secret"} {
		if strings.Contains(out.String()+errs.String()+m.View(), secret) {
			t.Fatal("secret reached comparison output")
		}
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeySpace}))
	if m.targets == nil || m.targets.defaults {
		t.Fatal("ambiguous matching cell attempted clear")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	// Even with one public match, p is always an explicit apply for secret rotation.
	if err := svc.Delete("first"); err != nil {
		t.Fatal(err)
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}))
	if m.statuses[0].Profile != "second" {
		t.Fatal("unique public match not recognized")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"}))
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil || !strings.Contains(m.confirm.Question, "ANTHROPIC_AUTH_TOKEN") {
		t.Fatal("secret reapply did not use redacted native preview")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	native, _ := os.ReadFile(claude.ProviderFile())
	if !bytes.Contains(native, []byte("fixture-new-secret")) {
		t.Fatal("explicit credential reapply was not written")
	}
}

func TestComparisonErrorsAndUnsupportedNativeProtocolStayVisible(t *testing.T) {
	svc, claude, _, _ := validationService(t)
	p := agent.ProviderProfile{Name: "work", BaseURL: "https://example.test", Model: "m"}
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(p.Name, "claude-code"); err != nil {
		t.Fatal(err)
	}
	writeProviderFixture(t, claude.ProviderState, "not JSON")
	if st := svc.Status()[0]; st.Err == "" {
		t.Fatal("unreadable ownership claimed known state")
	}
	writeProviderFixture(t, svc.Path(), "not JSON")
	if st := svc.Status()[0]; st.Err == "" || st.Profile != "" {
		t.Fatal("unreadable library claimed a profile match")
	}
	paths := core.PathsIn(t.TempDir())
	crush := agent.NewCrush(paths.Home)
	if err := crush.ApplyProvider(p, paths.BackupsDir()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(crush.ProviderFile())
	writeProviderFixture(t, crush.ProviderFile(), strings.ReplaceAll(string(data), "--type openai-compat", "--type unsupported-native-api"))
	live, active, err := crush.ReadProvider()
	if err != nil || !active || live.WireAPI != "unsupported-native-api" {
		t.Fatal("unknown native protocol was guessed")
	}
	if name, _ := matchProfile(compareProfiles(crush, live, true, []agent.ProviderProfile{p})); name != "" {
		t.Fatal("unknown protocol falsely matched chat")
	}
}
