package sessions

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

// resume suspende a TUI e executa o CLI de origem no diretório da sessão.
func (m Sessions) resume(s agent.Session) (Sessions, tea.Cmd) {
	argv, dir, ok := m.svc.ResumeCmd(s)
	if !ok {
		m.toast, m.toastErr = s.AgentName+" não suporta resume via CLI", true
		return m, nil
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		m.toast, m.toastErr = argv[0]+" não está no PATH", true
		return m, nil
	}
	if _, err := os.Stat(dir); err != nil {
		// pasta não existe — abre picker para o usuário corrigir
		return m.openDirPicker(s, dir), nil
	}
	return m, m.runResume(s, dir)
}

func (m Sessions) openDirPicker(s agent.Session, prefill string) Sessions {
	inp := components.NewInput()
	inp.SetValue(prefill)
	inp.Focus()
	m.dirInput = inp
	m.pendingResume = s
	m.mode = sessModeDir
	return m
}

func (m Sessions) runResume(s agent.Session, dir string) tea.Cmd {
	argv, _, _ := m.svc.ResumeCmd(s)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return resumeDoneMsg{err: err}
	})
}

func (m Sessions) updateDirPicker(msg tea.KeyPressMsg) (Sessions, tea.Cmd) {
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
			m.toast, m.toastErr = "pasta não encontrada: "+raw, true
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
