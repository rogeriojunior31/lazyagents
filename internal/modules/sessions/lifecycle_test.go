package sessions

import (
	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"os"
	"testing"
)

func TestAliasDraftFailurePendingAndDiscard(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	m := newTab(New(nil, paths), paths.Home)
	m.width, m.height = 40, 13
	s := agent.Session{AgentID: "claude-code", ID: "one", Alias: "before"}
	m = m.openAlias(s)
	m, _ = m.update(tea.PasteMsg{Content: " changed"})
	draft := m.aliasInput.Value()
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.discardAlias == nil {
		t.Fatal("alias discarded without decision")
	}
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != sessModeAlias || m.aliasInput.Value() != draft {
		t.Fatal("cancel lost alias")
	}
	var cmd tea.Cmd
	m, cmd = m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.writing || cmd == nil {
		t.Fatal("alias save did not capture input")
	}
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyEscape}, tea.PasteMsg{Content: "ignored"}} {
		var duplicate tea.Cmd
		m, duplicate = m.update(msg)
		if duplicate != nil || !m.writing || m.aliasInput.Value() != draft {
			t.Fatal("pending alias save repeated or escaped")
		}
	}
	m, _ = m.update(aliasDoneMsg{err: os.ErrPermission})
	if m.writing || m.mode != sessModeAlias || !m.aliasInput.Focused() || m.aliasInput.Value() != draft {
		t.Fatal("failed save lost alias draft")
	}
	m, _ = m.update(aliasDoneMsg{key: "claude-code:one", alias: draft})
	if m.mode != sessModeList || m.writing {
		t.Fatal("successful alias save did not finish")
	}
	m.sessions = []agent.Session{s}
	m, _ = m.update(events.SessionsLoaded{Err: os.ErrPermission})
	if len(m.sessions) != 1 || !m.loadErr {
		t.Fatal("failed refresh erased previous sessions")
	}
}
