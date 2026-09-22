// Package plugin é a aba proxy de um plugin externo: encaminha tamanho,
// teclas e eventos ao processo (internal/plugin) e mostra o último frame que
// ele mandou. Um Module por binário; o registro dinâmico vive em internal/app.
package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	plug "github.com/rogeriojunior31/lazyagents/internal/plugin"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Module implementa module.Module e module.Commander sobre um plug.Proc.
// proc == nil é o estado morto: a aba mostra o erro e `r`/:reload respawna.
type Module struct {
	svc  *plug.Service
	pl   plug.Plugin
	init plug.Msg // guardado para o respawn

	proc   *plug.Proc
	err    error
	stderr string

	view      string
	count     int
	capturing bool
	width     int
	height    int
	agents    []plug.Agent // reenviados após respawn
}

// New sobe o plugin de forma síncrona (a paleta e o título precisam do
// manifesto antes do Init). Falha vira estado morto, nunca nil.
func New(svc *plug.Service, pl plug.Plugin, init plug.Msg) *Module {
	m := &Module{svc: svc, pl: pl, init: init, count: -1}
	m.start()
	return m
}

// start (re)inicia o processo. ponytail: síncrono também no :reload, então um
// plugin quebrado segura a TUI até o timeout do handshake (3 s).
func (m *Module) start() {
	m.init.Width, m.init.Height = m.width, m.height
	m.proc, m.err = m.svc.Start(m.pl, m.init)
	m.view, m.count, m.capturing, m.stderr = "", -1, false, ""
	if m.proc != nil && m.agents != nil {
		m.send(plug.Msg{Type: "agents", Agents: m.agents})
	}
}

func (m *Module) send(msg plug.Msg) {
	if m.proc != nil {
		_ = m.proc.Send(msg) // falha fecha o processo; Events fecha e vira exitMsg
	}
}

// wait lê UM evento do plugin; re-armado a cada frameMsg (padrão do spinner).
func (m *Module) wait() tea.Cmd {
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

func (m *Module) ID() string { return m.pl.ID }

func (m *Module) Title() string {
	if m.proc != nil && m.proc.Manifest.Title != "" {
		return m.proc.Manifest.Title
	}
	return m.pl.ID
}

func (m *Module) Count() int      { return m.count }
func (m *Module) Capturing() bool { return m.proc != nil && m.capturing }
func (m *Module) ClearToast()     {}
func (m *Module) Init() tea.Cmd   { return m.wait() }

func (m *Module) Help() []module.HelpGroup {
	if m.proc == nil {
		return []module.HelpGroup{{Title: m.pl.ID, Keys: [][2]string{{"r", "reinicia o plugin"}}}}
	}
	groups := make([]module.HelpGroup, 0, len(m.proc.Manifest.Help))
	for _, g := range m.proc.Manifest.Help {
		groups = append(groups, module.HelpGroup{Title: g.Title, Keys: g.Keys})
	}
	return groups
}

func (m *Module) Commands() []module.Command {
	if m.proc == nil {
		return nil
	}
	cmds := make([]module.Command, 0, len(m.proc.Manifest.Commands))
	for _, c := range m.proc.Manifest.Commands {
		cmds = append(cmds, module.Command{Name: c.Name, Desc: c.Desc, Msg: commandMsg{id: m.pl.ID, name: c.Name}})
	}
	return cmds
}

func (m *Module) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.send(plug.Msg{Type: "resize", Width: msg.Width, Height: msg.Height})
	case tea.KeyPressMsg:
		if m.proc == nil {
			if msg.String() == "r" {
				m.start()
				return m.wait()
			}
			return nil
		}
		m.send(plug.Msg{Type: "key", Key: msg.String(), Text: msg.Text})
	case tea.PasteMsg:
		m.send(plug.Msg{Type: "paste", Text: msg.Content})
	case tea.MouseWheelMsg:
		m.send(plug.Msg{Type: "mouse", Mouse: &plug.Mouse{Kind: "wheel", X: msg.X, Y: msg.Y, Button: tea.Mouse(msg).String()}})
	case tea.MouseClickMsg:
		m.send(plug.Msg{Type: "mouse", Mouse: &plug.Mouse{Kind: "click", X: msg.X, Y: msg.Y, Button: tea.Mouse(msg).String()}})
	case events.AgentsDetected:
		m.agents = toDTO(msg.Agents)
		m.send(plug.Msg{Type: "agents", Agents: m.agents})
	case events.Reload:
		if m.proc == nil {
			m.start()
			return m.wait()
		}
		m.send(plug.Msg{Type: "reload"})
	case commandMsg:
		if msg.id == m.pl.ID {
			m.send(plug.Msg{Type: "command", Name: msg.name})
		}
	case frameMsg:
		if msg.id != m.pl.ID {
			return nil
		}
		switch msg.msg.Type {
		case "frame":
			m.view, m.capturing, m.count = plug.CleanView(msg.msg.View), msg.msg.Capturing, -1
			if msg.msg.Count != nil && *msg.msg.Count >= 0 {
				m.count = *msg.msg.Count
			}
		case "exec":
			return tea.Batch(m.wait(), m.execCmd(msg.msg))
		}
		return m.wait()
	case exitMsg:
		if msg.id == m.pl.ID {
			m.proc, m.err, m.stderr, m.capturing = nil, fmt.Errorf("plugin %s: %w", m.pl.ID, msg.err), msg.stderr, false
		}
	case execDoneMsg:
		if msg.id != m.pl.ID {
			return nil
		}
		res := plug.Msg{Type: "exec_result", ExecID: msg.execID, Code: msg.code, Stdout: msg.stdout, Stderr: msg.stderr}
		if msg.err != nil {
			res.Error = msg.err.Error()
		}
		m.send(res)
	}
	return nil
}

// execCmd roda o comando pedido pelo plugin: interativo suspende a TUI
// (tea.ExecProcess); senão roda em background com a saída capturada.
func (m *Module) execCmd(req plug.Msg) tea.Cmd {
	id, execID := m.pl.ID, req.ExecID
	fail := func(err error) tea.Cmd {
		return func() tea.Msg { return execDoneMsg{id: id, execID: execID, code: -1, err: err} }
	}
	if len(req.Argv) == 0 {
		return fail(errors.New("exec sem argv"))
	}
	if _, err := exec.LookPath(req.Argv[0]); err != nil {
		return fail(err)
	}
	cmd := exec.Command(req.Argv[0], req.Argv[1:]...)
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

// nonExit descarta o ExitError (o código já vai em Code); outros erros ficam.
func nonExit(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil
	}
	return err
}

// capped guarda no máximo plug.MaxLine bytes.
type capped struct{ bytes.Buffer }

func (c *capped) Write(b []byte) (int, error) {
	if room := plug.MaxLine - c.Len(); len(b) > room {
		c.Buffer.Write(b[:max(0, room)])
		return len(b), nil
	}
	return c.Buffer.Write(b)
}

func toDTO(agents []agent.Agent) []plug.Agent {
	out := make([]plug.Agent, 0, len(agents))
	for _, a := range agents {
		out = append(out, plug.Agent{ID: a.ID, Name: a.Name, Installed: a.Installed, Version: a.Version, ManagedDir: a.ManagedDir, ReadDirs: a.ReadDirs})
	}
	return out
}

func (m *Module) View() string {
	if m.proc == nil {
		var b strings.Builder
		b.WriteString(kit.StErr.Render(m.err.Error()) + "\n\n")
		b.WriteString(kit.StHint.Render("r ou :reload reinicia o plugin") + "\n")
		if tail := strings.TrimSpace(m.stderr); tail != "" {
			b.WriteString("\n" + kit.StHint.Render("stderr:") + "\n" + tail + "\n")
		}
		return b.String()
	}
	if m.view == "" {
		return kit.StHint.Render("aguardando o plugin…")
	}
	return m.view
}
