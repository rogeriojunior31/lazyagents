package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Tab) openDocCmd() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	return loadDocCmd(sel.Name, sel.Path)
}

func loadDocCmd(name, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
		if err != nil {
			return docMsg{err: fmt.Errorf("reading SKILL.md of %s: %w", name, err)}
		}
		return docMsg{name: name, path: path, content: string(data)}
	}
}

// editCmd suspende a TUI e abre o SKILL.md no $EDITOR (fallback vi).
func editCmd(name, path string) tea.Cmd {
	fields := strings.Fields(os.Getenv("EDITOR"))
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	args := append(fields[1:], filepath.Join(path, "SKILL.md"))
	c := exec.Command(fields[0], args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editDoneMsg{name: name, err: err}
	})
}

func (m Tab) updateDoc(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.mode = skModeList
		return m, nil
	case "e":
		return m, editCmd(m.docName, m.docPath)
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m Tab) updateNew(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}
		m.input.Blur()
		svc := m.svc
		return m, func() tea.Msg {
			path, err := svc.Create(name)
			return createdMsg{name: name, path: path, err: err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
