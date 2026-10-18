package recovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/modules/providers"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

type failingHistoryCodex struct{ *agent.Codex }

func (c *failingHistoryCodex) ApplyProvider(p agent.ProviderProfile, dir string) error {
	return fmt.Errorf("PRIVATE_COMMAND_AND_TOKEN")
}

func historyFixture(t *testing.T) (*Service, core.HistoryEntry, string) {
	t.Helper()
	paths := core.PathsIn(t.TempDir())
	c := agent.NewClaude(paths.Home)
	write(t, c.ProviderFile(), []byte(`{"foreign":"PRIVATE_BODY"}`))
	adapters := []agent.Adapter{c, &failingHistoryCodex{agent.NewCodex(paths.Home)}}
	svc := providers.New(adapters, paths)
	if err := svc.Save(agent.ProviderProfile{Name: "local", BaseURL: "https://example.test/v1", Token: "PRIVATE_TOKEN", EnvKey: "TEST_TOKEN", Model: "small"}); err != nil {
		t.Fatal(err)
	}
	result := svc.ApplyResult("local", []string{"claude-code", "codex"})
	if result.AsError() == nil || result.HistoryWarning != "" {
		t.Fatalf("partial result: %+v", result)
	}
	// A new service reads only persisted history after the attempt.
	restarted := New(adapters, paths.BackupsDir())
	entries, err := core.ReadHistory(restarted.paths)
	if err != nil || len(entries) != 1 || !entries[0].Failed || len(entries[0].Backups) != 1 {
		t.Fatalf("history=%+v err=%v", entries, err)
	}
	return restarted, entries[0], c.ProviderFile()
}

func TestHistoryRestartInspectAndClearCLI(t *testing.T) {
	svc, e, path := historyFixture(t)
	before, _ := os.ReadFile(path)
	var out, errOut bytes.Buffer
	c := cli.Context{Out: &out, Err: &errOut}
	if code := cmdHistory(c, []string{"list", "--query", "providers codex failed", "--json"}, svc); code != 0 {
		t.Fatalf("list=%d %s", code, &errOut)
	}
	var entries []core.HistoryEntry
	if json.Unmarshal(out.Bytes(), &entries) != nil || len(entries) != 1 {
		t.Fatalf("list output %s", &out)
	}
	out.Reset()
	if code := cmdHistory(c, []string{"inspect", e.ID, "--json"}, svc); code != 0 {
		t.Fatalf("inspect=%d %s", code, &errOut)
	}
	var p HistoryInspection
	if err := json.Unmarshal(out.Bytes(), &p); err != nil || len(p.Backups) != 1 || !p.Backups[0].Available {
		t.Fatalf("previewability %s err=%v", &out, err)
	}
	for _, secret := range []string{"PRIVATE_TOKEN", "PRIVATE_BODY", "PRIVATE_COMMAND_AND_TOKEN"} {
		if strings.Contains(out.String()+errOut.String(), secret) {
			t.Fatal("history leaked private data")
		}
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("inspection wrote config")
	}
	backupPath := filepath.Join(svc.dir, e.Backups[0].ID)
	if err := os.Remove(backupPath); err != nil {
		t.Fatal(err)
	}
	p, err := svc.inspectHistory(e.ID)
	if err != nil || p.Backups[0].Available {
		t.Fatal("missing backup still available")
	}
	if code := cmdHistory(c, []string{"clear"}, svc); code == 0 {
		t.Fatal("clear accepted without confirmation")
	}
	entries, _ = core.ReadHistory(svc.paths)
	if len(entries) != 1 {
		t.Fatal("unconfirmed clear wrote")
	}
	// Metadata file is a separate backup artifact and must survive clear.
	metadataPath := filepath.Join(svc.dir, ".config-backup-meta", e.Backups[0].ID+".json")
	if code := cmdHistory(c, []string{"clear", "--yes"}, svc); code != 0 {
		t.Fatalf("clear=%d %s", code, &errOut)
	}
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatal("clear deleted backup metadata")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("clear wrote config")
	}
}

func TestHistoryUIBoundsNavigationAndFreshBackupPreview(t *testing.T) {
	svc, e, path := historyFixture(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(m.Update(openHistoryMsg{})())
	if !m.Capturing() || len(m.history.entries) != 1 {
		t.Fatal("history not loaded")
	}
	m.Update(m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})())
	if m.history.detail == nil {
		t.Fatal("history not inspected")
	}
	before, _ := os.ReadFile(path)
	for _, width := range []int{36, 40, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 13})
		view := m.View()
		if lipgloss.Width(view) > width || lipgloss.Height(view) > 13 || !strings.Contains(view, "esc") || !strings.Contains(view, "review") || !strings.Contains(view, "preview") {
			t.Fatalf("history overflow %dx16: %s", width, view)
		}
	}
	// Mouse/paste cannot execute or dismiss a modal.
	m.Update(tea.PasteMsg{Content: "rbc"})
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft})
	if m.history.detail == nil {
		t.Fatal("input escaped history")
	}
	m.Update(m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})())
	if m.history != nil || m.preview == nil || m.preview.ID != e.Backups[0].ID || m.confirm != nil {
		t.Fatal("backup did not require fresh preview/confirmation")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("preview wrote config")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(m.Update(openHistoryMsg{})())
	m.Update(m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})())
	cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if event, ok := cmd().(events.OpenTab); !ok || event.ID != "providers" || m.history == nil {
		t.Fatal("review did not navigate to fresh provider state")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("review replayed a write")
	}
}

func TestHistoryClearCancelPendingAndLateMessages(t *testing.T) {
	svc, _, _ := historyFixture(t)
	m := newTab(svc)
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	cmd := m.Update(openHistoryMsg{})
	old := m.history
	m.Update(cmd())
	m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if m.history.confirm == nil {
		t.Fatal("clear skipped confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.history.confirm != nil || len(m.history.entries) != 1 {
		t.Fatal("escape cleared history")
	}
	m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	cmd = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.history == nil || !m.history.busy {
		t.Fatal("pending clear lost its owner")
	}
	m.Update(cmd())
	if len(m.history.entries) != 0 || m.history.busy {
		t.Fatal("clear did not finish")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(m.Update(openHistoryMsg{})())
	m.Update(historyLoadedMsg{view: old, entries: []core.HistoryEntry{{ID: "old"}}})
	if len(m.history.entries) != 0 {
		t.Fatal("late load replaced current history")
	}
}
