package skills

import (
	"context"
	"sync"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

// One foreground source operation owns its context, progress and staging.
// Commands complete before cancellation cleanup; the UI never removes a tree
// while its worker is still reading or replacing it.
type sourceOperation struct {
	ctx     context.Context
	cancel  context.CancelFunc
	events  chan string
	done    chan struct{}
	mu      sync.Mutex
	cleanup func()
	started bool
	back    skMode
}

type sourceProgressMsg struct {
	op    *sourceOperation
	label string
	more  bool
}

func (m *Tab) beginSource(label string, mode, back skMode) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.sourceOp = &sourceOperation{ctx: ctx, cancel: cancel, events: make(chan string, 1), done: make(chan struct{}), back: back}
	m.mode = mode
	return tea.Batch(m.beginSpin(label), m.sourceOp.wait())
}

func (o *sourceOperation) notify(label string) {
	select {
	case o.events <- label:
	default:
	}
}
func (o *sourceOperation) wait() tea.Cmd {
	return func() tea.Msg { label, ok := <-o.events; return sourceProgressMsg{o, label, ok} }
}
func (o *sourceOperation) own(cleanup func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cleanup = cleanup
}
func (o *sourceOperation) clean() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cleanup != nil {
		o.cleanup()
		o.cleanup = nil
	}
}
func (o *sourceOperation) release() { o.mu.Lock(); o.cleanup = nil; o.mu.Unlock() }
func (o *sourceOperation) command(work func(context.Context, func(string)) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		o.mu.Lock()
		o.started = true
		o.mu.Unlock()
		defer func() {
			if o.ctx.Err() != nil {
				o.clean()
			}
			close(o.events)
			close(o.done)
		}()
		return work(o.ctx, o.notify)
	}
}
func (o *sourceOperation) close() {
	if o == nil {
		return
	}
	o.cancel()
	o.mu.Lock()
	started := o.started
	o.mu.Unlock()
	if !started {
		o.clean()
		return
	}
	select {
	case <-o.done:
		o.clean()
	case <-time.After(3 * time.Second): // The worker retains cleanup ownership.
	}
}
func (m *Tab) finishSource() {
	if m.sourceOp != nil {
		m.sourceOp.release()
		m.sourceOp.cancel()
		m.sourceOp = nil
	}
	m.inFlight = false
}
func (m Tab) sourceCommand(work func(context.Context, func(string)) tea.Msg) tea.Cmd {
	if m.sourceOp != nil {
		return m.sourceOp.command(work)
	}
	return func() tea.Msg { return work(context.Background(), nil) }
}

func (m Tab) sourceBusyView(title string) string {
	w := min(m.width, 84)
	inner := max(1, w-4)
	footer := ""
	if m.sourceOp != nil {
		footer = kit.Wrap("esc cancel; wait for cleanup", inner, "")
	}
	if m.mode == skModeUpdateBusy {
		footer = kit.Wrap("esc cancel; completed items keep backups", inner, "")
	}
	vp := viewport.New(viewport.WithWidth(inner), viewport.WithHeight(max(1, m.height-3-lipgloss.Height(footer))))
	vp.SetContent(kit.Wrap(m.spin.View()+" "+reviewText(m.toast), inner, ""))
	return components.Panel{Title: title, Focused: true, Width: w}.Render(vp.View() + "\n" + footer)
}
