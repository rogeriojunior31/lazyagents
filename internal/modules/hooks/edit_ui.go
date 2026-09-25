package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type hookReader struct {
	hook             Hook
	index            int
	docs             []hookDocument
	selected, scroll int
	notice           string
}
type documentsMsg struct {
	reader *hookReader
	docs   []hookDocument
	err    error
}
type editSavedMsg struct {
	reader *hookReader
	hook   Hook
	docs   []hookDocument
	err    error
}
type editPreparedMsg struct {
	reader *hookReader
	doc    hookDocument
	path   string
	err    error
}
type hookEditedMsg struct {
	reader *hookReader
	doc    hookDocument
	text   string
	err    error
}

func (m *Tab) openReader() tea.Cmd {
	h, ok := m.current()
	if !ok || len(h.Hooks) == 0 {
		return nil
	}
	index := commandOrder(h)[min(m.cmdCursor, len(h.Hooks)-1)]
	reader := &hookReader{hook: h, index: index}
	m.reader = reader
	svc := m.svc
	return func() tea.Msg { docs, err := svc.documents(h, index); return documentsMsg{reader, docs, err} }
}

func (m *Tab) updateReader(msg tea.Msg) tea.Cmd {
	r := m.reader
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "q":
			m.reader = nil
			return nil
		case "left", "right":
			if len(r.docs) > 0 {
				d := 1
				if key.String() == "left" {
					d = -1
				}
				r.selected = (r.selected + d + len(r.docs)) % len(r.docs)
				r.scroll = 0
			}
			return nil
		case "e":
			if len(r.docs) == 0 {
				return nil
			}
			doc := r.docs[r.selected]
			return func() tea.Msg { path, err := editCopy(doc); return editPreparedMsg{r, doc, path, err} }
		}
	}
	vp := m.readerViewport()
	vp, _ = vp.Update(msg)
	r.scroll = vp.YOffset()
	return nil
}

func (m Tab) readerViewport() viewport.Model {
	r := m.reader
	text := "Loading command and scripts…"
	if len(r.docs) > 0 {
		var lines []string
		for i, line := range strings.Split(strings.ReplaceAll(ansi.Strip(r.docs[r.selected].Text), "\t", "    "), "\n") {
			lines = append(lines, fmt.Sprintf("%3d  %s", i+1, line))
		}
		text = strings.Join(lines, "\n")
	}
	w, h := max(1, m.width-5), max(1, m.height-4)
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(ansi.Wrap(text, w, ""))
	vp.SetYOffset(r.scroll)
	return vp
}

func (m Tab) readerView() string {
	r := m.reader
	vp := m.readerViewport()
	title := "READER"
	if len(r.docs) > 0 {
		label := "command"
		if r.docs[r.selected].Path != "" {
			label = filepath.Base(r.docs[r.selected].Path)
		}
		title = fmt.Sprintf("%d/%d · %.0f%% · %s", r.selected+1, len(r.docs), vp.ScrollPercent()*100, label)
	}
	rows := strings.Split(vp.View(), "\n")
	h := vp.Height()
	total := max(h, vp.TotalLineCount())
	thumb := max(1, h*h/total)
	top := int(float64(h-thumb) * vp.ScrollPercent())
	for i := range rows {
		mark := "│"
		if i >= top && i < top+thumb {
			mark = "┃"
		}
		rows[i] = lipgloss.NewStyle().Width(vp.Width()).Render(rows[i]) + kit.StShared.Render(mark)
	}
	p := components.Panel{Title: title, Focused: true, Width: m.width, Height: max(4, m.height-2)}
	foot := kit.Hints(m.width, [2]string{"e", "edit"}, [2]string{"←→", "file"}, [2]string{"↑↓", "scroll"}, [2]string{"esc", "back"})
	return lipgloss.JoinVertical(lipgloss.Left, p.Render(strings.Join(rows, "\n")), foot, kit.StHint.Render(ansi.Truncate(r.notice, max(1, m.width), "…")))
}

func (m *Tab) preparedEdit(msg editPreparedMsg) tea.Cmd {
	if m.reader != msg.reader {
		if msg.path != "" {
			return func() tea.Msg { _ = os.Remove(msg.path); return nil }
		}
		return nil
	}
	if msg.err != nil {
		m.reader.notice = msg.err.Error()
		return nil
	}
	fields := strings.Fields(os.Getenv("EDITOR"))
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	cmd := exec.Command(fields[0], append(fields[1:], msg.path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(msg.path)
		text := ""
		if err == nil {
			text, err = readScript(msg.path)
		}
		return hookEditedMsg{msg.reader, msg.doc, text, err}
	})
}

func (m *Tab) finishEdit(msg hookEditedMsg) tea.Cmd {
	if m.reader != msg.reader {
		return nil
	}
	r := m.reader
	if msg.err != nil {
		r.notice = "Editor: " + msg.err.Error()
		return nil
	}
	if msg.text == msg.doc.Text {
		r.notice = "No changes."
		return nil
	}
	question := fmt.Sprintf("Save changes to the command of %s?\nAlso updates the agents where this command is installed.", r.hook.Name)
	if msg.doc.Path != "" {
		question = fmt.Sprintf("Save changes to %s?\nHooks that use this file will run the new content.", msg.doc.Path)
	}
	svc := m.svc
	return m.ask(question+"\nBackup before writing.\n\nBEFORE\n"+ansi.Strip(msg.doc.Text)+"\n\nAFTER\n"+ansi.Strip(msg.text), func() tea.Msg {
		err := svc.saveDocument(r.hook, r.index, msg.doc, msg.text)
		if err != nil {
			return editSavedMsg{r, r.hook, r.docs, err}
		}
		h, err := svc.Get(r.hook.Name)
		if err != nil {
			return editSavedMsg{r, r.hook, r.docs, err}
		}
		docs, err := svc.documents(h, r.index)
		return editSavedMsg{r, h, docs, err}
	})
}
