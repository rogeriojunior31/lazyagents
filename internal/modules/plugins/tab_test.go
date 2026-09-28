package plugins

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins/plugintest"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

// newModule starts the tab of plugin "echo", the Go fake in the given mode
// (testdata/fakeplugin): "echo" proxies keys, commands and exec; "fail" dies.
func newModule(t *testing.T, mode string) *Tab {
	t.Helper()
	t.Setenv("FAKEPLUGIN_MODE", mode)
	svc := New(core.PathsIn(t.TempDir()))
	svc.Handshake = 2 * time.Second // the first run of a fresh binary can be slow on Windows
	t.Cleanup(svc.Close)
	path := plugintest.Install(t, svc.Dir, "echo")
	return newTab(svc, Plugin{ID: "echo", Path: path}, Msg{})
}

// run executes the module's cmd and feeds each msg to Update as it arrives,
// like the runtime (Batch children in parallel); returns the last non-nil cmd.
func run(t *testing.T, m *Tab, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil cmd: the module stopped listening to the plugin")
	}
	ch := make(chan tea.Msg, 16)
	spawn(cmd, ch)
	var next tea.Cmd
	for pending := 1; pending > 0; {
		select {
		case msg := <-ch:
			if sp, ok := msg.(spawned); ok {
				pending += sp.n - 1
				continue
			}
			pending--
			if c := m.Update(msg); c != nil {
				next = c
			}
		case <-time.After(3 * time.Second):
			t.Fatal("plugin did not answer")
		}
	}
	return next
}

// spawned tells run that a Batch became n cmds.
type spawned struct{ n int }

func spawn(cmd tea.Cmd, ch chan tea.Msg) {
	go func() {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				spawn(c, ch)
			}
			ch <- spawned{len(b)}
			return
		}
		ch <- msg
	}()
}

func TestModuleProxiesPlugin(t *testing.T) {
	m := newModule(t, "echo")
	if m.Title() != "Echo" || m.ID() != "echo" || len(m.Help()) != 1 || len(m.Commands()) != 1 {
		t.Fatalf("manifest not applied: title=%q help=%d cmds=%d", m.Title(), len(m.Help()), len(m.Commands()))
	}
	cmd := run(t, m, m.Init())
	if !strings.Contains(m.View(), "\x1b[1mhello ✓") || strings.Contains(m.View(), "[2J") || m.Count() != 3 {
		t.Errorf("initial frame: view=%q count=%d", m.View(), m.Count())
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	cmd = run(t, m, cmd)
	if m.View() != "key x" || !m.Capturing() {
		t.Errorf("echo: view=%q capturing=%v", m.View(), m.Capturing())
	}
	m.Update(m.Commands()[0].Msg) // palette → command → plugin asks for exec
	cmd = run(t, m, cmd)          // frame(exec) → Update returns Batch(wait, exec)
	cmd = run(t, m, cmd)          // exec runs → exec_result → final frame
	if m.View() != "exec code 2" {
		t.Errorf("exec: view=%q", m.View())
	}
	m.Update(events.Reload{}) // the fake exits 0
	run(t, m, cmd)
	if m.proc != nil || !strings.Contains(m.View(), "plugin echo:") || m.Capturing() {
		t.Errorf("expected the dead state: %q", m.View())
	}
	// r respawns
	cmd = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	run(t, m, cmd)
	if m.proc == nil || m.Count() != 3 {
		t.Errorf("respawn failed: %v", m.err)
	}
}

func TestModuleDeadOnStartFailure(t *testing.T) {
	m := newModule(t, "fail")
	if m.proc != nil || m.Init() != nil || m.Title() != "echo" || m.Commands() != nil {
		t.Fatalf("should start dead: %+v", m)
	}
	if v := m.View(); !strings.Contains(v, "plugin echo:") || !strings.Contains(v, "failed") {
		t.Errorf("dead state view: %q", v)
	}
	if cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"}); cmd != nil {
		t.Error("a key in the dead state should not produce a cmd")
	}
}

func TestExecStopsWhenServiceCloses(t *testing.T) {
	m := newModule(t, "echo")
	cmd := m.execCmd(Msg{Argv: []string{plugintest.Build(t), "sleep-child"}})
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	m.svc.Close()
	select {
	case <-result:
	case <-time.After(4 * time.Second):
		t.Fatal("exec survived service close")
	}
}

func TestLongFailureCanBeReadAndRestarted(t *testing.T) {
	m := newModule(t, "echo")
	cmd := run(t, m, m.Init())
	m.Update(events.Reload{})
	run(t, m, cmd)
	m.stderr = strings.Repeat("long diagnostic\n", 40) + "END_ERROR\x1b[2J"
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 9})
	if !strings.Contains(m.View(), "restart") || m.Count() != -1 {
		t.Fatal("a failure must offer a restart and clear the count")
	}
	for i := 0; i < 50; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if v := m.View(); !strings.Contains(v, "END_ERROR") || strings.Contains(v, "[2J") || !strings.Contains(v, "restart") {
		t.Fatalf("end of error unreachable or control not sanitized: %q", v)
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 14})
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	run(t, m, m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}))
	if m.proc == nil || m.scroll != 0 || m.Count() != 3 {
		t.Fatal("restart did not restore the plugin")
	}
}
