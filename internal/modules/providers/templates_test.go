package providers

import (
	"bytes"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func chooseTemplate(t *testing.T, m *Tab, name string) {
	t.Helper()
	m.Update(newProfileMsg{})
	for i, tmpl := range connectionTemplates {
		if tmpl.name == name {
			for range i {
				m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return
		}
	}
	t.Fatalf("missing template %q", name)
}

func TestConnectionTemplatesMatchSupportedNativeProtocols(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	adapters := agent.All(paths.Home)
	for _, tmpl := range connectionTemplates {
		t.Run(tmpl.name, func(t *testing.T) {
			if _, err := time.Parse("2006-01-02", tmpl.reviewed); err != nil {
				t.Fatal(err)
			}
			source, err := url.Parse(tmpl.source)
			if err != nil || source.Scheme != "https" || source.Host == "" || len(tmpl.agents) == 0 || tmpl.required == "" || tmpl.credential == "" || tmpl.protocol == "" {
				t.Fatalf("incomplete guidance: %+v", tmpl)
			}
			if tmpl.profile.Token != "" || tmpl.profile.HasToken || tmpl.profile.Model != "" {
				t.Fatal("template contains credentials or assumes a model")
			}
			if tmpl.defaults || tmpl.profile.Name == "" {
				return
			}
			p := tmpl.profile
			p.Model = "test-tool-model"
			if p.BaseURL == "" {
				p.BaseURL = "https://gateway.example.test"
				p.Token = "test-gateway-bearer"
			}
			for _, id := range tmpl.agents {
				host := agent.ByID(adapters, id).(agent.ProviderHost)
				if _, err := host.(agent.ProviderValidator).ValidateProvider(p); err != nil {
					t.Fatalf("%s template is invalid for %s: %v", tmpl.name, id, err)
				}
				if err := host.ApplyProvider(p, paths.BackupsDir()); err != nil {
					t.Fatal(err)
				}
				got, active, err := host.ReadProvider()
				if err != nil || !active || got.BaseURL != p.BaseURL || got.Model != p.Model || got.Token != "" {
					t.Fatalf("%s native readback: %+v %v", id, got, err)
				}
				if p.EnvKey != "" && got.EnvKey != p.EnvKey {
					t.Fatalf("lost credential reference for %s", id)
				}
				if p.Token == "" && p.EnvKey == "" && got.HasToken {
					t.Fatalf("keyless template gained a credential for %s", id)
				}
			}
		})
	}
}

func TestTemplateFormSavesEditableValuesWithoutApplying(t *testing.T) {
	svc, claude, codex, _ := validationService(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	chooseTemplate(t, &m, "OpenAI Responses")
	if m.form == nil || m.form.focus != fModel || m.form.profile().EnvKey != "OPENAI_API_KEY" || m.form.profile().WireAPI != "responses" {
		t.Fatalf("template did not fill the form: %+v", m.form)
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.form == nil || !strings.Contains(m.form.err, "model") {
		t.Fatal("missing model was saved")
	}
	typeText(&m, "test-model")
	m.form.setFocus(fName)
	m.form.inputs[fName].SetValue("customized")
	m.form.setFocus(fBaseURL)
	m.form.inputs[fBaseURL].SetValue("https://proxy.example.test/v1")
	m.form.setFocus(fEnvKey)
	m.form.inputs[fEnvKey].SetValue("CUSTOM_API_KEY")
	run(t, &m, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	p, err := svc.Profile("customized")
	if err != nil || p.Model != "test-model" || p.BaseURL != "https://proxy.example.test/v1" || p.EnvKey != "CUSTOM_API_KEY" || p.WireAPI != "responses" {
		t.Fatalf("saved = %+v, %v", p.Redacted(), err)
	}
	for _, path := range []string{claude.ProviderFile(), codex.ProviderFile()} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("saving template wrote %s", path)
		}
	}
	data, _ := os.ReadFile(svc.Path())
	if bytes.Contains(data, []byte(`"template"`)) || bytes.Contains(data, []byte(`"source"`)) {
		t.Fatal("template metadata became a new profile format")
	}
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if m.form == nil || m.form.template != nil || m.form.profile().BaseURL != p.BaseURL {
		t.Fatal("editing a saved profile did not keep custom values")
	}
}

func TestTemplateCancellationAndGatewayCredential(t *testing.T) {
	svc, claude, _, _ := validationService(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(newProfileMsg{})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Capturing() {
		t.Fatal("chooser did not close")
	}
	chooseTemplate(t, &m, "Anthropic gateway for Claude Code")
	if m.form == nil || m.form.focus != fBaseURL {
		t.Fatal("missing gateway endpoint should be focused")
	}
	m.form.inputs[fBaseURL].SetValue("https://gateway.example.test")
	m.form.inputs[fModel].SetValue("gateway-model")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.form.focus != fToken || !strings.Contains(m.form.err, "bearer token") {
		t.Fatal("gateway did not require its bearer credential")
	}
	typeText(&m, "secret-never-display")
	if strings.Contains(m.View(), "secret-never-display") {
		t.Fatal("typed token leaked")
	}
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}))
	if m.targets == nil || !m.targets.inspect {
		t.Fatal("template did not use shared draft validation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.form.inputs[fToken].Value() != "secret-never-display" {
		t.Fatal("checking discarded the typed token")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	for _, path := range []string{svc.Path(), claude.ProviderFile()} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("cancel/check wrote %s", path)
		}
	}
}

func TestNativeDefaultsOnlyClearExplicitSelections(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	adapters := []agent.Adapter{agent.NewClaude(paths.Home), agent.NewCodex(paths.Home), agent.NewPi(paths.Home), agent.NewCrush(paths.Home)}
	svc := New(adapters, paths)
	svc.Detect = func() []agent.Agent {
		return []agent.Agent{{ID: "claude-code", Name: "Claude Code", Installed: true}, {ID: "codex", Name: "Codex", Installed: true}, {ID: "pi", Name: "Pi", Installed: true}, {ID: "crush", Name: "Crush", Installed: true}}
	}
	if err := svc.Save(agent.ProviderProfile{Name: "existing", BaseURL: "https://example.test", Model: "test-model", Token: "secret-never-display", EnvKey: "TEST_KEY"}); err != nil {
		t.Fatal(err)
	}
	if r := svc.ApplyResult("existing", nil); r.AsError() != nil {
		t.Fatal(r.AsError())
	}
	before, _ := os.ReadFile(svc.Path())
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	chooseTemplate(t, &m, "Agent defaults / previous settings")
	if m.targets == nil || !m.targets.defaults || len(m.targets.selected) != 0 {
		t.Fatal("defaults must require an explicit selection")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm != nil {
		t.Fatal("empty native selection reached confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // Pi only, including both native files
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirm == nil || !strings.Contains(m.confirm.Question, "native/previous") || strings.Contains(m.confirm.Question, "\nCodex\n") {
		t.Fatal("defaults confirmation did not match selection")
	}
	files := agent.ProviderFiles(adapters[2].(agent.ProviderHost))
	if len(files) != 2 {
		t.Fatal("Pi review must include both native destinations")
	}
	for _, file := range files {
		if !strings.Contains(m.confirm.Question, core.Tilde(file, paths.Home)) {
			t.Fatalf("missing clear destination %s", file)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, active, _ := adapters[2].(agent.ProviderHost).ReadProvider(); !active {
		t.Fatal("cancel cleared a provider")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	if m.report == nil || m.report.Result.AsError() != nil || len(m.report.Result.Targets) != 1 {
		t.Fatalf("clear report = %+v", m.report)
	}
	for i, ad := range adapters {
		_, active, err := ad.(agent.ProviderHost).ReadProvider()
		if err != nil || active != (i != 2) {
			t.Fatalf("%s active=%v err=%v", ad.ID(), active, err)
		}
	}
	after, _ := os.ReadFile(svc.Path())
	if !bytes.Equal(before, after) {
		t.Fatal("native defaults changed or created a library profile")
	}
}

func TestTemplateAndFocusedHelpLayouts(t *testing.T) {
	for _, size := range [][2]int{{36, 8}, {40, 14}, {80, 24}, {120, 11}, {220, 32}} {
		m := newTab(New(nil, core.PathsIn(t.TempDir())))
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.Update(newProfileMsg{})
		for i := range connectionTemplates {
			m.templates.cursor = i
			v := m.View()
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] || !strings.Contains(ansi.Strip(v), "esc") {
				t.Fatalf("chooser at %v:\n%s", size, ansi.Strip(v))
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		for i := range fieldCount {
			m.form.setFocus(i)
			v := m.View()
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] || !strings.Contains(ansi.Strip(v), "About "+fieldLabels[i]) || !strings.Contains(ansi.Strip(v), "esc") || !strings.Contains(ansi.Strip(v), "save") {
				t.Fatalf("field %d at %v:\n%s", i, size, ansi.Strip(v))
			}
		}
		old := m.form.profile()
		for range 120 {
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		if (m.form.helpScroll == 0 && size[1] < 30) || m.form.profile() != old || !strings.Contains(ansi.Strip(m.View()), "check.") {
			t.Fatalf("field guidance is not scrollable at %v:\n%s", size, ansi.Strip(m.View()))
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.form.helpScroll != 0 {
			t.Fatal("field change did not reset help")
		}
	}
}
