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
	draft            *hookEditedMsg
	discard          *components.Confirm
	editing, loaded  bool
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
	committed bool
	reader    *hookReader
	hook      Hook
	docs      []hookDocument
	err       error
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
	if r.editing {
		return nil
	}
	if r.discard != nil {
		c, answer := r.discard.Update(msg, m.width, m.height)
		r.discard = &c
		if answer == components.No {
			r.discard = nil
		}
		if answer == components.Yes {
			m.reader = nil
		}
		return nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "q":
			if r.draft != nil {
				c := components.NewConfirm("Discard the unsaved hook edit?\nChoose No to keep it for e (edit) or s (review save). Installed files are unchanged by discarding.")
				r.discard = &c
				return nil
			}
			m.reader = nil
			return nil
		case "left", "right":
			if r.draft != nil {
				r.notice = "Save or discard the unsaved edit before switching files"
				return nil
			}
			if len(r.docs) > 0 {
				d := 1
				if key.String() == "left" {
					d = -1
				}
				r.selected = (r.selected + d + len(r.docs)) % len(r.docs)
				r.scroll = 0
			}
			return nil
		case "s":
			if r.draft != nil {
				return m.finishEdit(*r.draft)
			}
			return nil
		case "e":
			if len(r.docs) == 0 || r.editing {
				return nil
			}
			doc := r.docs[r.selected]
			copyDoc := doc
			if r.draft != nil {
				doc = r.draft.doc
				copyDoc = doc
				copyDoc.Text = r.draft.text
			}
			r.editing = true
			return func() tea.Msg { path, err := editCopy(copyDoc); return editPreparedMsg{r, doc, path, err} }
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
	if r.loaded {
		text = "No readable command documents; close and reopen to retry"
	}
	if len(r.docs) > 0 {
		var lines []string
		source := r.docs[r.selected].Text
		if r.draft != nil {
			source = r.draft.text
		}
		for i, line := range strings.Split(strings.ReplaceAll(ansi.Strip(source), "\t", "    "), "\n") {
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
	if r.discard != nil {
		return r.discard.ViewIn(m.width, m.height)
	}
	vp := m.readerViewport()
	title := "READER"
	if len(r.docs) > 0 {
		label := "command"
		if r.docs[r.selected].Path != "" {
			label = filepath.Base(r.docs[r.selected].Path)
		}
		title = fmt.Sprintf("%d/%d · %.0f%% · %s", r.selected+1, len(r.docs), vp.ScrollPercent()*100, label)
	}
	if r.draft != nil {
		title = "Unsaved · " + title
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
	if r.draft != nil {
		foot = kit.Hints(m.width, [2]string{"e", "edit"}, [2]string{"s", "save"}, [2]string{"esc", "discard"})
	}
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
		m.reader.editing = false
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
		text, readErr := readScript(msg.path)
		if readErr != nil {
			text = msg.doc.Text
			if err == nil {
				err = readErr
			}
		}
		return hookEditedMsg{msg.reader, msg.doc, text, err}
	})
}

func (m *Tab) finishEdit(msg hookEditedMsg) tea.Cmd {
	if m.reader != msg.reader {
		return nil
	}
	r := m.reader
	r.editing = false
	if msg.err != nil {
		r.notice = "Editor: " + msg.err.Error()
		if msg.text != msg.doc.Text {
			msg.err = nil
			r.draft = &msg
			r.notice = "Editor failed; edited content kept. Use e to resume or s to review saving"
		}
		return nil
	}
	if msg.text == msg.doc.Text {
		r.draft = nil
		r.notice = "No changes."
		return nil
	}
	r.draft = &msg
	question := fmt.Sprintf("Save changes to the command of %s?\nAlso updates the agents where this command is installed.", r.hook.Name)
	if msg.doc.Path != "" {
		question = fmt.Sprintf("Save changes to %s?\nHooks that use this file will run the new content.", msg.doc.Path)
	}
	svc := m.svc
	return m.ask(question+"\nBackup before writing.\n\nBEFORE\n"+ansi.Strip(msg.doc.Text)+"\n\nAFTER\n"+ansi.Strip(msg.text), func() tea.Msg {
		err := svc.saveDocument(r.hook, r.index, msg.doc, msg.text)
		if err != nil {
			return editSavedMsg{reader: r, hook: r.hook, docs: r.docs, err: err}
		}
		h, err := svc.Get(r.hook.Name)
		if err != nil {
			return editSavedMsg{committed: true, reader: r, hook: r.hook, docs: r.docs, err: err}
		}
		docs, err := svc.documents(h, r.index)
		return editSavedMsg{committed: true, reader: r, hook: h, docs: docs, err: err}
	})
}
