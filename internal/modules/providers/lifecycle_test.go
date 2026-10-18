package providers

import (
	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"os"
	"strings"
	"testing"
)

func TestProfileDraftDiscardPendingSaveAndFailure(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	svc := New(nil, paths)
	m := newTab(svc)
	m.width, m.height = 40, 13
	m.form = newProfileForm(agent.ProviderProfile{Name: "draft", BaseURL: "https://example.test/v1"}, false)
	f := m.form
	f.setFocus(fToken)
	f.inputs[fToken].SetValue("typed-private-token")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if f.discard == nil {
		t.Fatal("edited profile discarded without a decision")
	}
	m.Update(tea.PasteMsg{Content: "ignored"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.form != f || f.inputs[fToken].Value() != "typed-private-token" || f.focus != fToken {
		t.Fatal("cancel lost draft/focus")
	}
	if strings.Contains(m.View(), "typed-private-token") {
		t.Fatal("typed credential revealed")
	}
	if err := os.MkdirAll(svc.path, 0o700); err != nil {
		t.Fatal(err)
	}
	save := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || !m.writing {
		t.Fatal("save did not own input")
	}
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyEscape}, tea.PasteMsg{Content: "changed"}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}, tea.MouseClickMsg{Button: tea.MouseLeft}} {
		if m.Update(msg) != nil || m.form != f || !m.writing {
			t.Fatal("pending save escaped/repeated")
		}
	}
	m.Update(save())
	if m.writing || m.form != f || f.err == "" || f.inputs[fToken].Value() != "typed-private-token" {
		t.Fatal("write failure lost draft")
	}
	if err := os.Remove(svc.path); err != nil {
		t.Fatal(err)
	}
	save = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(save())
	if m.form != nil || m.writing {
		t.Fatal("successful save did not complete")
	}
	m.form = newProfileForm(agent.ProviderProfile{Name: "another"}, false)
	current := m.form
	m.Update(savedMsg{form: f, name: "stale"})
	if m.form != current {
		t.Fatal("late save closed a newer form")
	}
	if p, _ := svc.Profile("draft"); p.Token != "typed-private-token" {
		t.Fatal("explicit save lost token")
	}
	fresh := newProfileForm(agent.ProviderProfile{Name: "draft", HasToken: true}, true)
	if fresh.inputs[fToken].Value() != "" || fresh.dirty() {
		t.Fatal("saved credential restored into form")
	}
}

func TestFailedProviderReloadPreservesPreviousEvidence(t *testing.T) {
	m := newTab(New(nil, core.PathsIn(t.TempDir())))
	m.width, m.height = 80, 24
	m.Update(loadedMsg{profiles: []agent.ProviderProfile{{Name: "keep"}}})
	m.detailOff = 3
	m.Update(loadedMsg{err: os.ErrPermission})
	if len(m.profiles) != 1 || m.profiles[0].Name != "keep" || m.detailOff != 3 || m.loadErr == "" {
		t.Fatal("failed load erased previous evidence")
	}
	m.Update(loadedMsg{profiles: []agent.ProviderProfile{{Name: "keep"}}})
	if m.loadErr != "" {
		t.Fatal("fresh load stayed failed")
	}
}
