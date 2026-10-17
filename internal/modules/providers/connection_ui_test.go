package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestCatalogSelectionEditsFormWithoutSavingOrApplying(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fixture-saved-secret" {
			t.Error("form lost saved credential")
		}
		w.Write([]byte(`{"data":[{"id":"model-one"},{"id":"model-two"}]}`))
	}))
	defer server.Close()
	svc, claude, codex, _ := validationService(t)
	if err := svc.Save(agent.ProviderProfile{Name: "catalog", BaseURL: server.URL + "/v1", Token: "fixture-saved-secret", Model: "manual"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(svc.Path())
	m := newTab(svc)
	defer m.closeConnection()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	run(t, &m, m.Init())
	run(t, &m, m.Update(events.TabActivated{ID: "providers"}))
	if calls.Load() != 0 {
		t.Fatal("network ran at boot/activation")
	}
	// Matrix discovery opens an edit form rather than persisting the selected ID.
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"}))
	if m.connection == nil || !m.connection.result.Catalog || m.connection.source.Token != "" {
		t.Fatal("catalog state missing or contains credential")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.form == nil || m.form.orig != "catalog" || m.form.inputs[fToken].Value() != "" || m.form.profile().Model != "model-two" || !m.form.hasToken {
		t.Fatal("catalog selection did not open a safe edit draft")
	}
	// Form discovery resolves the stored token without loading it into an input.
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}))
	m.Update(tea.KeyPressMsg{Code: 'o', Text: "one"})
	if ids := m.connection.models(); len(ids) != 1 || ids[0] != "model-one" {
		t.Fatalf("filter failed: %v", ids)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.form.profile().Model != "model-one" || m.form.inputs[fToken].Value() != "" {
		t.Fatal("selected model lost or token materialized in form")
	}
	after, _ := os.ReadFile(svc.Path())
	if !bytes.Equal(before, after) || calls.Load() != 2 {
		t.Fatal("selection saved or made extra requests")
	}
	for _, path := range []string{claude.ProviderFile(), codex.ProviderFile(), claude.ProviderState} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("network action wrote native config/state")
		}
	}
}

func TestCatalogCancellationStopsIOAndIgnoresLateResults(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(stopped) }))
	defer server.Close()
	svc, _, _, _ := validationService(t)
	m := newTab(svc)
	defer m.closeConnection()
	m.form = newProfileForm(agent.ProviderProfile{Name: "draft", BaseURL: server.URL, Model: "manual", Token: ""}, false)
	m.form.inputs[fToken].SetValue("typed-fixture-secret")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	<-started
	oldRequest := m.connection.request
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("escape did not cancel network IO")
	}
	select {
	case msg := <-result:
		m.Update(msg)
	case <-time.After(time.Second):
		t.Fatal("cancelled request did not finish")
	}
	if m.connection != nil || m.form.profile().Model != "manual" || m.form.inputs[fToken].Value() != "typed-fixture-secret" {
		t.Fatal("cancellation discarded draft or reopened result")
	}
	// A reply from the old request cannot replace the new pending action.
	m.form.inputs[fBaseURL].SetValue("not a URL")
	next := m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m.Update(connectionMsg{request: oldRequest, result: ConnectionResult{Catalog: true, Models: []string{"stale"}}})
	if m.connection == nil || !m.connection.busy {
		t.Fatal("late reply replaced current action")
	}
	run(t, &m, next)
	if m.connection == nil || m.connection.result.Error == "" {
		t.Fatal("invalid draft check did not return evidence")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.form.profile().Model != "manual" {
		t.Fatal("failed check changed manual model")
	}
}

func TestConnectionPanelsFitAndKeepEvidenceScrollable(t *testing.T) {
	svc, _, _, _ := validationService(t)
	for _, size := range [][2]int{{36, 8}, {40, 14}, {80, 24}, {120, 11}, {220, 32}} {
		m := newTab(svc)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.form = newProfileForm(agent.ProviderProfile{Name: "draft", BaseURL: "http://localhost:1234", Model: "manual"}, false)
		m.form.inputs[fToken].SetValue("never-display-fixture-secret")
		m.connectionCmd(true) // Not executed: layout checks must not make network requests.
		for _, state := range []string{"busy", "catalog", "error", "probe"} {
			p := m.connection
			p.busy = state == "busy"
			p.result = ConnectionResult{Operation: "models", Reachable: true, HTTPStatus: 200, Credential: "sent", ModelStatus: "listed"}
			switch state {
			case "catalog":
				p.result.Catalog = true
				p.result.Models = []string{"model-one", "model-two", "very-long-model-" + strings.Repeat("x", 100)}
			case "error":
				p.result.Error = "No supported model catalog at this base URL; enter the model manually"
			}
			view := m.View()
			if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] || !strings.Contains(ansi.Strip(view), "esc") || strings.Contains(view, "never-display-fixture-secret") {
				t.Fatalf("%s at %v:\n%s", state, size, ansi.Strip(view))
			}
			if state == "catalog" {
				p.result.Models = nil
				if view := m.View(); lipgloss.Height(view) > size[1] || !strings.Contains(ansi.Strip(view), "No matching") {
					t.Fatalf("empty catalog at %v:\n%s", size, ansi.Strip(view))
				}
			}
		}
		for range 100 {
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		if !strings.Contains(ansi.Strip(m.View()), "settings changed.") {
			t.Fatalf("evidence cannot scroll to final limits at %v:\n%s", size, ansi.Strip(m.View()))
		}
		encoded, _ := json.Marshal(m.connection.result)
		if strings.Contains(string(encoded), "never-display") {
			t.Fatal("secret in result")
		}
		m.closeConnection()
	}
}

func TestDraftCredentialPrecedenceAndProbeNeverResolvesSecrets(t *testing.T) {
	auth := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth <- r.Header.Get("Authorization")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	svc, _, _, _ := validationService(t)
	p := agent.ProviderProfile{Name: "saved", BaseURL: server.URL, Token: "saved-fixture-secret"}
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	draft := p.Redacted()
	draft.Token = "typed-fixture-secret"
	if r := svc.CheckDraft(context.Background(), draft, p.Name, true); !r.Catalog || <-auth != "Bearer typed-fixture-secret" {
		t.Fatal("typed token did not take precedence")
	}
	draft.Token = ""
	if r := svc.CheckDraft(context.Background(), draft, p.Name, true); !r.Catalog || <-auth != "Bearer saved-fixture-secret" {
		t.Fatal("saved editing token was not resolved")
	}
	// A stale saved-token marker and unset env reference do not affect credential-free probes.
	draft.EnvKey = "UNSET_NETWORK_TEST_KEY"
	t.Setenv(draft.EnvKey, "")
	if r := svc.CheckDraft(context.Background(), draft, "deleted-profile", false); !r.Reachable || r.Credential != "not_sent" || <-auth != "" {
		t.Fatal("probe fetched or sent credentials")
	}
	if r := svc.CheckDraft(context.Background(), draft, "deleted-profile", true); r.Error == "" || r.Reachable {
		t.Fatal("stale saved credential silently fell back")
	}
	select {
	case <-auth:
		t.Fatal("stale credential caused a request")
	default:
	}
}
