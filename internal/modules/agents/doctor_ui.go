package agents

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

type doctorRequest struct {
	ctx     context.Context
	cancel  context.CancelFunc
	started atomic.Bool
	done    chan struct{}
}
type doctorState struct {
	run             cli.DoctorRunner
	open            bool
	request         *doctorRequest
	report          cli.DoctorReport
	section, offset int
}
type openDoctorMsg struct{}
type doctorDoneMsg struct {
	request *doctorRequest
	report  cli.DoctorReport
}

func (m *Tab) startDoctor(external bool) tea.Cmd {
	if m.doctor.run == nil {
		return nil
	}
	if m.doctor.request != nil {
		m.doctor.open = true
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	req := &doctorRequest{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	scope := "local"
	if external {
		scope = "all"
	}
	m.doctor.open, m.doctor.request = true, req
	m.doctor.report = cli.DoctorReport{Scope: scope}
	m.doctor.section, m.doctor.offset = 0, 0
	run := m.doctor.run
	return func() tea.Msg {
		req.started.Store(true)
		defer close(req.done)
		defer cancel()
		if ctx.Err() != nil {
			return doctorDoneMsg{req, cli.DoctorReport{Scope: scope, Cancelled: true}}
		}
		return doctorDoneMsg{req, run(ctx, external)}
	}
}

func (m *Tab) closeDoctor() {
	if m.doctor.request != nil {
		m.doctor.request.cancel()
	}
	m.doctor.open = false
}

func (m *Tab) shutdownDoctor() {
	m.closeDoctor()
	req := m.doctor.request
	if req == nil || !req.started.Load() {
		return
	}
	select {
	case <-req.done:
	case <-time.After(3 * time.Second): // The cancelled worker retains process cleanup.
	}
}

func (m *Tab) updateDoctor(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "q":
			m.closeDoctor()
			return nil
		case "r":
			return m.startDoctor(false)
		case "n":
			return m.startDoctor(true)
		case "left", "h", "right", "l":
			delta := 1
			if key.String() == "left" || key.String() == "h" {
				delta = -1
			}
			m.doctor.section = max(0, min(m.doctor.section+delta, len(m.doctor.report.Sections)-1))
			m.doctor.offset = 0
			return nil
		case "enter":
			sections := m.doctor.report.Sections
			if len(sections) > 0 && sections[m.doctor.section].Tab != "" {
				tab := sections[m.doctor.section].Tab
				m.closeDoctor()
				return func() tea.Msg { return events.OpenTab{ID: tab} }
			}
		}
	}
	vp := m.doctorViewport()
	vp, _ = vp.Update(msg)
	m.doctor.offset = vp.YOffset()
	return nil
}

func (m Tab) doctorHints() string {
	return kit.Wrap("←→ section · ↑↓/pgdn scroll\nr local · n usage/plugins\nenter open tab · esc back", max(1, m.width-4), "")
}

func (m Tab) doctorHeader() string {
	if m.doctor.request != nil {
		if m.doctor.request.ctx.Err() != nil {
			return "Cancelling checks; waiting for cleanup"
		}
		return "Running checks; esc cancels and returns to Agents"
	}
	scope := "Local checks · usage/plugins not run"
	if m.doctor.report.Scope == "all" {
		scope = "Local and external checks"
	}
	if m.doctor.report.Cancelled {
		return scope + " · cancelled; retry before relying on this report"
	}
	count := 0
	for _, s := range m.doctor.report.Sections {
		count += len(s.Problems)
	}
	return fmt.Sprintf("%s · %d problem(s)", scope, count)
}

func (m Tab) doctorViewport() viewport.Model {
	inner := max(1, m.width-4)
	head := kit.Wrap(m.doctorHeader(), inner, "")
	h := max(1, m.height-2-lipgloss.Height(head)-lipgloss.Height(m.doctorHints()))
	vp := viewport.New(viewport.WithWidth(inner), viewport.WithHeight(h))
	content := "Checking configuration and feature diagnostics…"
	sections := m.doctor.report.Sections
	if len(sections) > 0 {
		s := sections[m.doctor.section]
		lines := []string{fmt.Sprintf("%d/%d · %s", m.doctor.section+1, len(sections), s.Title)}
		switch {
		case s.Skipped:
			lines = append(lines, s.Report, "Next: press n to include usage/plugins, or r to rerun local checks.")
		case len(s.Findings) > 0:
			for _, p := range s.Findings {
				heading := strings.ToUpper(p.Severity)
				if !strings.Contains(p.Explanation, p.Resource) {
					heading += " · " + p.Resource
				}
				lines = append(lines, "", heading, p.Explanation, "Next: "+p.NextStep)
			}
			if s.External && s.Report != "" {
				lines = append(lines, "", "Check output:", s.Report)
			}
		default:
			text := s.Report
			if text == "" {
				text = "No problems found in this check."
			}
			lines = append(lines, text)
		}
		content = strings.Join(lines, "\n")
	}
	// Plugin output, paths and file metadata must never control the terminal.
	content = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.In(r, unicode.Cf)) {
			return unicode.ReplacementChar
		}
		return r
	}, content)
	vp.SetContent(kit.Wrap(content, inner, ""))
	vp.SetYOffset(m.doctor.offset)
	return vp
}

func (m Tab) doctorView() string {
	inner := max(1, m.width-4)
	body := kit.Wrap(m.doctorHeader(), inner, "") + "\n" + m.doctorViewport().View() + "\n" + m.doctorHints()
	return components.Panel{Title: "Doctor", Focused: true, Width: m.width}.Render(body)
}

func (m *Tab) Commands() []module.Command {
	return []module.Command{
		{Name: "doctor", Desc: "review local diagnostics; external checks run only on request", Msg: openDoctorMsg{}},
		{Name: "refresh", Desc: "refresh agent detection across tabs without network queries", Msg: refreshAgentsMsg{}},
	}
}
