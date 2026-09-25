package plugins

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Tab is the proxy tab of an external plugin: forwards size, keys and events to
// the process and shows its last frame. One per binary, built by this package's
// Feature; implements module.Module and module.Commander over a Proc.
// proc == nil is the dead state: the tab shows the error and r/:reload respawns.
type Tab struct {
	svc  *Service
	pl   Plugin
	init Msg // kept for respawn

	proc   *Proc
	err    error
	stderr string
	scroll int

	view      string
	count     int
	capturing bool
	width     int
	height    int
	agents    []Agent // resent after respawn
}

// newTab starts the plugin synchronously (palette and title need the manifest
// before Init). Failure becomes the dead state, never nil.
func newTab(svc *Service, pl Plugin, init Msg) *Tab {
	m := &Tab{svc: svc, pl: pl, init: init, count: -1}
	m.start()
	return m
}

// start (re)starts the process. ponytail: synchronous on :reload too, so a broken
// plugin holds the TUI until the handshake timeout (3 s).
func (m *Tab) start() {
	m.init.Width, m.init.Height = m.width, m.height
	m.proc, m.err = m.svc.Start(m.pl, m.init)
	m.scroll = 0
	m.view, m.count, m.capturing, m.stderr = "", -1, false, ""
	if m.proc != nil && m.agents != nil {
		m.send(Msg{Type: "agents", Agents: m.agents})
	}
}

func (m *Tab) send(msg Msg) {
	if m.proc != nil {
		_ = m.proc.Send(msg) // failure closes the process; Events closes and becomes exitMsg
	}
}

// wait reads ONE plugin event; re-armed on every frameMsg (spinner pattern).
func (m *Tab) wait() tea.Cmd {
	p, id := m.proc, m.pl.ID
	if p == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-p.Events
		if !ok {
			return exitMsg{id: id, err: p.Err(), stderr: p.StderrTail()}
		}
		return frameMsg{id: id, msg: ev}
	}
}

func (m *Tab) ID() string { return m.pl.ID }

func (m *Tab) Title() string {
	if m.proc != nil && m.proc.Manifest.Title != "" {
		return m.proc.Manifest.Title
	}
	return m.pl.ID
}

func (m *Tab) Count() int      { return m.count }
func (m *Tab) Capturing() bool { return m.proc != nil && m.capturing }
func (m *Tab) ClearToast()     {}
func (m *Tab) Init() tea.Cmd   { return m.wait() }

func (m *Tab) Help() []module.HelpGroup {
	if m.proc == nil {
		return []module.HelpGroup{{Title: m.pl.ID, Keys: [][2]string{{"r", "restart the plugin"}, {"↑/↓ · pgup/pgdn", "scroll the error"}}}}
	}
	groups := make([]module.HelpGroup, 0, len(m.proc.Manifest.Help))
	for _, g := range m.proc.Manifest.Help {
		groups = append(groups, module.HelpGroup{Title: g.Title, Keys: g.Keys})
	}
	return groups
}

func (m *Tab) Commands() []module.Command {
	if m.proc == nil {
		return nil
	}
	cmds := make([]module.Command, 0, len(m.proc.Manifest.Commands))
	for _, c := range m.proc.Manifest.Commands {
		cmds = append(cmds, module.Command{Name: c.Name, Desc: c.Desc, Msg: commandMsg{id: m.pl.ID, name: c.Name}})
	}
	return cmds
}

func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.send(Msg{Type: "resize", Width: msg.Width, Height: msg.Height})
	case tea.KeyPressMsg:
		if m.proc == nil {
			if msg.String() == "r" {
				m.start()
				return m.wait()
			}
			m.scrollError(msg)
			return nil
		}
		m.send(Msg{Type: "key", Key: msg.String(), Text: msg.Text})
	case tea.PasteMsg:
		m.send(Msg{Type: "paste", Text: msg.Content})
	case tea.MouseWheelMsg:
		if m.proc == nil {
			m.scrollError(msg)
			return nil
		}
		m.send(Msg{Type: "mouse", Mouse: &Mouse{Kind: "wheel", X: msg.X, Y: msg.Y, Button: tea.Mouse(msg).String()}})
	case tea.MouseClickMsg:
		m.send(Msg{Type: "mouse", Mouse: &Mouse{Kind: "click", X: msg.X, Y: msg.Y, Button: tea.Mouse(msg).String()}})
	case events.AgentsDetected:
		m.agents = toDTO(msg.Agents)
		m.send(Msg{Type: "agents", Agents: m.agents})
	case events.Reload:
		if m.proc == nil {
			m.start()
			return m.wait()
		}
		m.send(Msg{Type: "reload"})
	case commandMsg:
		if msg.id == m.pl.ID {
			m.send(Msg{Type: "command", Name: msg.name})
		}
	case frameMsg:
		if msg.id != m.pl.ID {
			return nil
		}
		switch msg.msg.Type {
		case "frame":
			m.view, m.capturing, m.count = CleanView(msg.msg.View), msg.msg.Capturing, -1
			if msg.msg.Count != nil && *msg.msg.Count >= 0 {
				m.count = *msg.msg.Count
			}
		case "exec":
			return tea.Batch(m.wait(), m.execCmd(msg.msg))
		}
		return m.wait()
	case exitMsg:
		if msg.id == m.pl.ID {
			m.scroll, m.count = 0, -1
			m.proc, m.err, m.stderr, m.capturing = nil, fmt.Errorf("plugin %s: %w", m.pl.ID, msg.err), msg.stderr, false
		}
	case execDoneMsg:
		if msg.id != m.pl.ID {
			return nil
		}
		res := Msg{Type: "exec_result", ExecID: msg.execID, Code: msg.code, Stdout: msg.stdout, Stderr: msg.stderr}
		if msg.err != nil {
			res.Error = msg.err.Error()
		}
		m.send(res)
	}
	return nil
}

// execCmd runs the command the plugin asked for: interactive suspends the TUI
// (tea.ExecProcess); otherwise it runs in the background with captured output.
func (m *Tab) execCmd(req Msg) tea.Cmd {
	id, execID := m.pl.ID, req.ExecID
	fail := func(err error) tea.Cmd {
		return func() tea.Msg { return execDoneMsg{id: id, execID: execID, code: -1, err: err} }
	}
	if len(req.Argv) == 0 {
		return fail(errors.New("exec without argv"))
	}
	if _, err := exec.LookPath(req.Argv[0]); err != nil {
		return fail(err)
	}
	cmd := exec.CommandContext(m.svc.ctx, req.Argv[0], req.Argv[1:]...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = m.svc.Env()
	cmd.Dir = req.Dir
	if req.Interactive {
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return execDoneMsg{id: id, execID: execID, code: exitCode(err), err: nonExit(err)}
		})
	}
	return func() tea.Msg {
		out, errb := &capped{}, &capped{}
		cmd.Stdout, cmd.Stderr = out, errb
		err := cmd.Run()
		return execDoneMsg{id: id, execID: execID, code: exitCode(err), stdout: out.String(), stderr: errb.String(), err: nonExit(err)}
	}
}

func exitCode(err error) int {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	}
	return -1
}

// nonExit drops ExitError (the code is already in Code); other errors stay.
func nonExit(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil
	}
	return err
}

// capped keeps at most MaxLine bytes.
type capped struct{ bytes.Buffer }

func (c *capped) Write(b []byte) (int, error) {
	if room := MaxLine - c.Len(); len(b) > room {
		c.Buffer.Write(b[:max(0, room)])
		return len(b), nil
	}
	return c.Buffer.Write(b)
}

func toDTO(agents []agent.Agent) []Agent {
	out := make([]Agent, 0, len(agents))
	for _, a := range agents {
		out = append(out, Agent{ID: a.ID, Name: a.Name, Installed: a.Installed, Version: a.Version, ManagedDir: a.ManagedDir, ReadDirs: a.ReadDirs})
	}
	return out
}

func (m *Tab) View() string {
	if m.proc == nil {
		vp := m.errorViewport()
		w := vp.Width()
		title := "Plugin unavailable"
		if vp.TotalLineCount() > vp.VisibleLineCount() {
			title += fmt.Sprintf(" · %.0f%%", vp.ScrollPercent()*100)
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			kit.StErr.Render(ansi.Truncate(title, w, "…")), vp.View(),
			kit.Hints(w, [2]string{"r", "restart"}, [2]string{"↑↓", "scroll error"}, [2]string{"?", "help"}))
	}
	if m.view == "" {
		return kit.StHint.Render("waiting for the plugin…")
	}
	return m.view
}

// Diagnostics belong to the host; live plugin frames stay untouched.
func (m *Tab) errorViewport() viewport.Model {
	message := "The plugin process exited."
	if m.err != nil {
		message = CleanView(m.err.Error())
	}
	if tail := strings.TrimSpace(CleanView(m.stderr)); tail != "" {
		message += "\n\nstderr:\n" + tail
	}
	w := m.width
	if w <= 0 {
		w = max(1, lipgloss.Width(message))
	}
	message = ansi.Wrap(message, w, "")
	h := max(1, m.height-2)
	if m.height <= 0 {
		h = lipgloss.Height(message)
	}
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(message)
	vp.SetYOffset(m.scroll)
	return vp
}

func (m *Tab) scrollError(msg tea.Msg) {
	vp := m.errorViewport()
	vp, _ = vp.Update(msg)
	m.scroll = vp.YOffset()
}
