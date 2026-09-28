package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func TestReadAndEditHookDocuments(t *testing.T) {
	svc, home := testService(t)
	root := filepath.Join(home, "plugin with spaces")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "check.sh")
	original := "#!/bin/sh\n" + strings.Repeat("echo review\n", 50) + "# END_SCRIPT\n"
	if err := os.WriteFile(path, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	h := Hook{Name: "pack", Files: root, Hooks: []agent.Hook{{Event: agent.HookStop, Command: `sh "$CLAUDE_PLUGIN_ROOT/check.sh"`, Async: true, Timeout: 30}}}
	if err := svc.Save(h); err != nil {
		t.Fatal(err)
	}
	docs, err := svc.documents(h, 0)
	if err != nil || len(docs) != 2 || docs[1].Text != original {
		t.Fatalf("read: %+v %v", docs, err)
	}
	if err := svc.Enable(h.Name, "claude-code"); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 11})
	run(t, &m, m.Update(events.TabActivated{ID: "hooks"}))
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"}))
	if m.reader == nil || m.reader.selected != 1 {
		t.Fatal("v did not open the script")
	}
	for i := 0; i < 100; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	view := m.View()
	if !strings.Contains(view, "END_SCRIPT") || !strings.Contains(view, "100%") || !strings.Contains(view, "┃") || lipgloss.Height(view) > 11 || lipgloss.Width(view) > 36 {
		t.Fatalf("bad reader view:\n%s", view)
	}
	edited := original + "# edited\n"
	m.Update(hookEditedMsg{m.reader, docs[1], edited, nil})
	if m.confirm == nil {
		t.Fatal("edit must ask for confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // not the default
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatal("default confirm changed the file")
	}
	m.Update(hookEditedMsg{m.reader, docs[1], edited, nil})
	run(t, &m, m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}))
	data, _ = os.ReadFile(path)
	st, _ := os.Stat(path)
	if string(data) != edited || (runtime.GOOS != "windows" && st.Mode().Perm() != 0755) || m.reader.docs[1].Text != edited {
		t.Fatal("edit lost content, mode or reader update")
	}
	if err := svc.saveDocument(h, 0, docs[1], "stale"); err == nil {
		t.Fatal("overwrote an external change")
	}
	backups, _ := os.ReadDir(svc.backupsDir)
	if len(backups) == 0 {
		t.Fatal("missing backup")
	}
	command := "sh \"" + path + "\" --check"
	if err := svc.saveDocument(h, 0, docs[0], command); err != nil {
		t.Fatal(err)
	}
	updated, _ := svc.Get(h.Name)
	installed, err := agent.NewClaude(home).ReadHooks()
	if err != nil || !containsHook(installed, updated.Hooks[0]) || containsHook(installed, h.Hooks[0]) || !updated.Hooks[0].Async || updated.Hooks[0].Timeout != 30 {
		t.Fatalf("command not updated: %+v %v", installed, err)
	}
	if err := svc.saveDocument(h, 0, docs[0], "  "); err == nil {
		t.Fatal("accepted an empty command")
	}
	if err := svc.saveDocument(h, 0, docs[0], "echo stale"); err == nil {
		t.Fatal("accepted a stale edit")
	}
}

func TestScriptInspectionDoesNotExecuteShell(t *testing.T) {
	svc, home := testService(t)
	marker := filepath.Join(home, "executed")
	h := Hook{Hooks: []agent.Hook{{Command: "echo $(touch " + marker + ")"}}}
	docs, err := svc.documents(h, 0)
	if err != nil || len(docs) != 1 {
		t.Fatalf("%v %v", docs, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("inspection executed the command")
	}
	p := filepath.Join(home, "binary.sh")
	os.WriteFile(p, []byte{0, 1, 2}, 0600)
	if _, err := readScript(p); err == nil {
		t.Fatal("accepted a binary")
	}
}

// Fails after adding the new command: the edit must restore the original.
type editFailureHost struct {
	agent.Adapter
	agent.HooksHost
	failRemove string
}

func (h *editFailureHost) RemoveHook(one agent.Hook, backup string) error {
	if one.Command == h.failRemove {
		h.failRemove = ""
		return errors.New("simulated removal failure")
	}
	return h.HooksHost.RemoveHook(one, backup)
}
func TestCommandEditRollsBackAgentFailure(t *testing.T) {
	for _, existingNew := range []bool{false, true} {
		svc, home := testService(t)
		claude := agent.NewClaude(home)
		old := agent.Hook{Event: agent.HookStop, Command: "echo old"}
		next := old
		next.Command = "echo new"
		h := Hook{Name: "rollback", Hooks: []agent.Hook{old}}
		if err := svc.Save(h); err != nil {
			t.Fatal(err)
		}
		if err := claude.AddHook(old, svc.backupsDir); err != nil {
			t.Fatal(err)
		}
		if existingNew {
			if err := claude.AddHook(next, svc.backupsDir); err != nil {
				t.Fatal(err)
			}
		}
		svc.adapters = []agent.Adapter{&editFailureHost{Adapter: claude, HooksHost: claude, failRemove: old.Command}}
		err := svc.saveDocument(h, 0, hookDocument{Text: old.Command}, next.Command)
		if err == nil {
			t.Fatal("expected a failure")
		}
		installed, _ := claude.ReadHooks()
		stored, _ := svc.Get(h.Name)
		if !containsHook(installed, old) || containsHook(installed, next) != existingNew || stored.Hooks[0] != old {
			t.Fatalf("rollback did not keep the state: %+v %+v", installed, stored)
		}
	}
}
