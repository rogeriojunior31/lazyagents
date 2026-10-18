package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func doctorFixture(external bool) cli.DoctorReport {
	return cli.DoctorReport{Scope: map[bool]string{false: "local", true: "all"}[external], Sections: []cli.DoctorSection{
		{Title: "links", Tab: "skills", Problems: []string{"broken"}, Findings: []cli.Problem{{Severity: "warning", Resource: "/fixture/" + strings.Repeat("long-folder/", 10), Explanation: "Broken link\x1b]52;payload\a\u202e", NextStep: strings.Repeat("Inspect this resource and restore the target. ", 15) + "END_NEXT_STEP"}}},
		{Title: "usage", External: true, Skipped: !external, Report: "External query not run; request it explicitly."},
	}}
}

func TestDoctorOpensLocallyAndRequiresExplicitExternalChecks(t *testing.T) {
	m := newTab("/fixture", nil)
	calls := []bool{}
	m.doctor.run = func(ctx context.Context, external bool) cli.DoctorReport {
		calls = append(calls, external)
		return doctorFixture(external)
	}
	if m.Init() != nil || len(calls) != 0 {
		t.Fatal("checks ran on tab construction")
	}
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 13})
	cmd := m.Update(openDoctorMsg{})
	if cmd == nil || !m.Capturing() || len(calls) != 0 {
		t.Fatal("opening did synchronous I/O")
	}
	if m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"}) != nil {
		t.Fatal("overlapping checks accepted")
	}
	m.Update(cmd())
	if len(calls) != 1 || calls[0] || !strings.Contains(m.doctorHeader(), "not run") {
		t.Fatal("local report claimed external success")
	}
	original := m.doctor.report.Sections[0].Findings[0].Resource
	m.doctor.report.Sections[0].Findings[0].Resource = "safe"
	text := m.doctorViewport().View()
	if !strings.Contains(text, "payload") || strings.Contains(text, "\x1b]52") || strings.Contains(text, "\u202e") {
		t.Fatalf("unsafe diagnostic content: %q", text)
	}
	m.doctor.report.Sections[0].Findings[0].Resource = original
	for range 200 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "END_NEXT_STEP") || lipgloss.Width(m.View()) > 36 || lipgloss.Height(m.View()) > 13 {
		t.Fatalf("long diagnostic unreachable at 40x16 root size:\n%s", plain)
	}
	if strings.Contains(m.doctorViewport().View(), "\x1b]52") || strings.Contains(m.doctorViewport().View(), "\u202e") {
		t.Fatal("untrusted terminal controls survived")
	}
	cmd = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	m.Update(cmd())
	if len(calls) != 2 || !calls[1] || m.doctor.report.Sections[1].Skipped {
		t.Fatal("explicit external checks missing")
	}
	m.doctor.report.Sections[1].Findings = []cli.Problem{{Severity: "warning", Resource: "plugin", Explanation: "Plugin reported a problem.", NextStep: "Inspect its output and retry."}}
	m.doctor.report.Sections[1].Report = "Detailed external failure: HTTP 503"
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	for range 40 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if !strings.Contains(ansi.Strip(m.View()), "503") {
		t.Fatal("external failure output was hidden")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Capturing() {
		t.Fatal("navigation kept the modal open")
	}
	if target := cmd().(events.OpenTab); target.ID != "skills" {
		t.Fatal(target)
	}
	cmd = m.Update(openDoctorMsg{})
	m.Update(cmd())
	cmd = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(cmd())
	if calls[len(calls)-1] {
		t.Fatal("local retry repeated remote calls")
	}
}

func TestDoctorCloseCancelsAndIgnoresLateResults(t *testing.T) {
	m := newTab("/fixture", nil)
	started := make(chan struct{})
	done := make(chan tea.Msg, 1)
	m.doctor.run = func(ctx context.Context, external bool) cli.DoctorReport {
		close(started)
		<-ctx.Done()
		return doctorFixture(external)
	}
	cmd := m.Update(openDoctorMsg{})
	go func() { done <- cmd() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	select {
	case msg := <-done:
		m.Update(msg)
		m.doctor.run = func(context.Context, bool) cli.DoctorReport { return cli.DoctorReport{OK: true, Scope: "local"} }
		next := m.Update(openDoctorMsg{})
		m.Update(msg)
		if m.doctor.request == nil || len(m.doctor.report.Sections) != 0 {
			t.Fatal("late result replaced current run")
		}
		m.Update(next())
		if !m.doctor.report.OK {
			t.Fatal("current result lost")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closing did not cancel the worker")
	}
	m.closeDoctor()
}

func TestDoctorShutdownDoesNotStartPendingIO(t *testing.T) {
	m := newTab("/fixture", nil)
	m.doctor.run = func(context.Context, bool) cli.DoctorReport {
		t.Fatal("shutdown started pending diagnostics")
		return cli.DoctorReport{}
	}
	cmd := m.Update(openDoctorMsg{})
	m.shutdownDoctor()
	m.Update(cmd())
	if m.doctor.request != nil || m.doctor.open || !m.doctor.report.Cancelled {
		t.Fatal("pending request was not cancelled")
	}
}
