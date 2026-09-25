package sessions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

// resume suspende a TUI e executa o CLI de origem no diretório da sessão.
func (m Tab) resume(s agent.Session) (Tab, tea.Cmd) {
	argv, dir, ok := m.svc.ResumeCmd(s)
	if !ok {
		m.toast, m.toastErr = fmt.Sprintf("%s does not support resume via CLI", s.AgentName), true
		return m, nil
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		m.toast, m.toastErr = fmt.Sprintf("%s is not in PATH", argv[0]), true
		return m, nil
	}
	if _, err := os.Stat(dir); err != nil {
		// pasta não existe — abre picker para o usuário corrigir
		return m.openDirPicker(s, dir), nil
	}
	return m, m.runResume(s, dir)
}

func (m Tab) openDirPicker(s agent.Session, prefill string) Tab {
	inp := components.NewInput()
	inp.SetValue(prefill)
	inp.Focus()
	m.dirInput = inp
	m.pendingResume = s
	m.mode = sessModeDir
	return m
}

func (m Tab) runResume(s agent.Session, dir string) tea.Cmd {
	argv, _, _ := m.svc.ResumeCmd(s)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return resumeDoneMsg{err: err}
	})
}

func (m Tab) updateDirPicker(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = sessModeList
		return m, nil
	case "enter":
		raw := strings.TrimSpace(m.dirInput.Value())
		if strings.HasPrefix(raw, "~/") {
			raw = filepath.Join(m.home, raw[2:])
		} else if raw == "~" {
			raw = m.home
		}
		if _, err := os.Stat(raw); err != nil {
			m.toast, m.toastErr = fmt.Sprintf("folder not found: %s", raw), true
			return m, nil
		}
		m.mode = sessModeList
		return m, m.runResume(m.pendingResume, raw)
	default:
		var cmd tea.Cmd
		m.dirInput, cmd = m.dirInput.Update(msg)
		return m, cmd
	}
}
