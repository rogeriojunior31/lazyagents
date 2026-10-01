package sessions

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

func TestProjectOf(t *testing.T) {
	cases := []struct {
		cwd  string
		want string
	}{
		{"", "no project"},
		{"/", "no project"},
		{"/tmp/lazyagents", "lazyagents"},
		{"/tmp/lazyagents/", "lazyagents"},
	}
	for _, c := range cases {
		if got := projectOf(agent.Session{CWD: c.cwd}); got != c.want {
			t.Errorf("projectOf(%q) = %q, want %q", c.cwd, got, c.want)
		}
	}
}

func TestRenderTranscriptEmpty(t *testing.T) {
	out := renderTranscript(nil, 80, agent.Session{}, transcriptOpts{}).content
	if !strings.Contains(out, "empty") {
		t.Errorf("empty transcript should show a hint, got:\n%s", out)
	}
}

// Consecutive agent messages form one turn; prompts are indexed for n/N.
func TestRenderTranscriptTurns(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "how do I do X in Go?"},
		{Role: agent.RoleThinking, Text: "first understand the error\nthen fix it"},
		{Role: agent.RoleAssistant, Text: "Let me look."},
		{Role: agent.RoleTool, Text: "Bash · ls"},
		{Role: agent.RoleTool, Text: "Bash · go test"},
		{Role: agent.RoleTool, Text: "Read · a.go"},
		{Role: agent.RoleAssistant, Text: "Use the `os` package."},
		{Role: agent.RoleUser, Text: "thanks"},
	}
	s := agent.Session{AgentID: "claude-code", AgentName: "Claude Code"}
	v := renderTranscript(entries, 60, s, transcriptOpts{})
	out := ansi.Strip(v.content)

	if n := strings.Count(out, "Claude Code"); n != 1 {
		t.Errorf("agent turns = %d, want 1 (grouped)\n%s", n, out)
	}
	if !strings.Contains(out, "#1 You ──") || !strings.Contains(out, "#2 You ──") {
		t.Errorf("prompts not numbered:\n%s", out)
	}
	// Folded: the steps on one line, then only the final reply.
	if !strings.Contains(out, "│ ⋯ 1 message · 1 thought · 3 commands · Bash ×2, Read") {
		t.Errorf("steps not folded:\n%s", out)
	}
	if strings.Contains(out, "Let me look") || strings.Contains(out, "first understand") {
		t.Errorf("narration and reasoning should be folded:\n%s", out)
	}
	if !strings.Contains(out, "│ Use the `os` package.") || !strings.Contains(out, "│ thanks") {
		t.Errorf("reply and prompt should sit behind a gutter:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	if len(v.prompts) != 2 || !strings.HasPrefix(lines[v.prompts[1]], "#2 You") {
		t.Errorf("prompts = %v", v.prompts)
	}
	if v.stats != (transcriptStats{prompts: 2, tools: 3, thoughts: 1}) {
		t.Errorf("stats = %+v", v.stats)
	}
	if w := lipgloss.Width(v.content); w > 60 {
		t.Errorf("width = %d, should not exceed 60", w)
	}

	// e: every step, collapsed reasoning and summarized commands.
	ev := renderTranscript(entries, 60, s, transcriptOpts{steps: true})
	out = ansi.Strip(ev.content)
	if !strings.Contains(out, "💭 first understand the error…") || strings.Contains(out, "then fix it") {
		t.Errorf("reasoning should be collapsed:\n%s", out)
	}
	if !regexp.MustCompile(`error…\n\s*│\n\s*│ Let me look`).MatchString(out) {
		t.Errorf("no blank line between reasoning and reply:\n%s", out)
	}
	if !strings.Contains(out, "❯ 3 commands · Bash ×2, Read") {
		t.Errorf("commands not summarized:\n%s", out)
	}
	if ev.stats != v.stats {
		t.Errorf("stats change with folding: %+v vs %+v", ev.stats, v.stats)
	}

	// t and r unfold too: one command per line and the full reasoning.
	out = ansi.Strip(renderTranscript(entries, 60, s, transcriptOpts{tools: true, thinking: true}).content)
	if !strings.Contains(out, "❯ Bash  go test") || strings.Contains(out, "commands ·") {
		t.Errorf("commands should be one per line:\n%s", out)
	}
	if !strings.Contains(out, "┆ first understand the error") || !strings.Contains(out, "┆ then fix it") {
		t.Errorf("reasoning should be full:\n%s", out)
	}
}

// A turn that never answered in text (interrupted, only commands) folds to its steps.
func TestRenderTranscriptTurnWithoutReply(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "run the tests"},
		{Role: agent.RoleTool, Text: "Bash · go test ./..."},
	}
	out := ansi.Strip(renderTranscript(entries, 60, agent.Session{}, transcriptOpts{}).content)
	if !strings.Contains(out, "│ ⋯ 1 command · Bash") {
		t.Errorf("steps line missing:\n%s", out)
	}
	// A single reply has nothing to fold.
	entries[1] = agent.Entry{Role: agent.RoleAssistant, Text: "done"}
	out = ansi.Strip(renderTranscript(entries, 60, agent.Session{}, transcriptOpts{}).content)
	if strings.Contains(out, "⋯") || !strings.Contains(out, "│ done") {
		t.Errorf("single reply should show alone:\n%s", out)
	}
}

func TestRenderTranscriptCapsWidthAndCenters(t *testing.T) {
	// Very wide terminal: the chat stops at maxChatWidth, centered.
	entries := []agent.Entry{{Role: agent.RoleAssistant, Text: strings.Repeat("word ", 80)}}
	out := ansi.Strip(renderTranscript(entries, 300, agent.Session{}, transcriptOpts{}).content)
	_, pad := chatColumn(300)
	for _, ln := range strings.Split(out, "\n") {
		if ln == "" {
			continue
		}
		if !strings.HasPrefix(ln, strings.Repeat(" ", pad)) {
			t.Fatalf("line without the %d-column indent: %q", pad, ln)
		}
		if w := lipgloss.Width(strings.TrimLeft(ln, " ")); w > maxChatWidth {
			t.Errorf("width = %d, should not exceed %d", w, maxChatWidth)
		}
	}
}

func TestAliasInTitleAndFilter(t *testing.T) {
	s := agent.Session{AgentID: "claude-code", ID: "1", Title: "first prompt", CWD: "/p/proj", Alias: "migration"}
	it := newSessionItem(s, false)
	if !strings.Contains(it.FilterValue(), "migration") || !strings.Contains(it.FilterValue(), "first prompt") {
		t.Errorf("FilterValue = %q", it.FilterValue())
	}
	if !strings.Contains(it.Title(), "migration") {
		t.Errorf("Title = %q", it.Title())
	}
}

// Every message after the last command is the answer, not only the last one.
func TestRenderTranscriptReplyAfterLastCommand(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "plan it"},
		{Role: agent.RoleAssistant, Text: "Researching."},
		{Role: agent.RoleTool, Text: "Agent · survey"},
		{Role: agent.RoleAssistant, Text: "# Plan"},
		{Role: agent.RoleAssistant, Text: "Waiting for your answers."},
	}
	out := ansi.Strip(renderTranscript(entries, 60, agent.Session{}, transcriptOpts{}).content)
	if strings.Contains(out, "Researching") || !strings.Contains(out, "⋯ 1 message · 1 command · Agent") {
		t.Errorf("narration before the command should fold:\n%s", out)
	}
	if !strings.Contains(out, "│ # Plan") || !strings.Contains(out, "│ Waiting for your answers.") {
		t.Errorf("messages after the last command should show:\n%s", out)
	}
}

// Events sit between turns, prompts carry their time and turns their duration,
// and a failed command is marked.
func TestRenderTranscriptTimesEventsFailures(t *testing.T) {
	at := time.Date(2026, 10, 1, 9, 4, 0, 0, time.Local)
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "run it", Time: at},
		{Role: agent.RoleTool, Text: "Bash · go test", Time: at.Add(time.Minute), Failed: true},
		{Role: agent.RoleTool, Text: "Bash · go vet", Time: at.Add(2 * time.Minute)},
		{Role: agent.RoleAssistant, Text: "One failure.", Time: at.Add(4*time.Minute + 12*time.Second)},
		{Role: agent.RoleEvent, Text: "context compacted"},
		{Role: agent.RoleUser, Text: "again", Time: at.Add(time.Hour)},
		{Role: agent.RoleAssistant, Text: "ok"},
		{Role: agent.RoleUser, Text: "next day", Time: at.Add(24 * time.Hour)},
	}
	s := agent.Session{AgentName: "Claude Code"}
	v := renderTranscript(entries, 60, s, transcriptOpts{})
	out := ansi.Strip(v.content)
	for _, want := range []string{
		"#1 You ", " 2026-10-01 09:04", // first prompt: date and hour
		" 10:04", // same day: hour only
		"2026-10-02 09:04",
		"── context compacted ──",
		"4m12s",
		"⋯ 2 commands · ✗ 1 failed · Bash ×2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "10-01 10:04") {
		t.Errorf("same day should show the hour only:\n%s", out)
	}
	if strings.Contains(out, "compacted ── ") {
		t.Errorf("event line padded on the right:\n%s", out)
	}
	out = ansi.Strip(renderTranscript(entries, 60, s, transcriptOpts{tools: true}).content)
	if !strings.Contains(out, "❯ Bash  go test ✗") {
		t.Errorf("failed command not marked:\n%s", out)
	}
}

// Calls read by what they did; a plan is part of the answer even folded.
func TestRenderTranscriptToolKinds(t *testing.T) {
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "plan and do it"},
		{Role: agent.RoleTool, Text: "Bash · go test ./...", Kind: agent.ToolShell, Failed: true},
		{Role: agent.RoleTool, Text: "Edit · /home/u/p/a.go", Kind: agent.ToolEdit, Added: 2, Removed: 1},
		{Role: agent.RoleTool, Text: "apply_patch · /p/a.go, /p/b.go", Kind: agent.ToolEdit, Added: 3},
		{Role: agent.RoleTool, Text: "Read · /p/a.go", Kind: agent.ToolRead},
		{Role: agent.RoleTool, Text: "Agent · Survey adapters", Kind: agent.ToolAgent},
		{Role: agent.RoleTool, Text: "TodoWrite", Kind: agent.ToolTodo, Body: "☑ write\n◐ test"},
		{Role: agent.RoleTool, Text: "ExitPlanMode", Kind: agent.ToolPlan, Body: "# Plan\nStep one."},
	}
	s := agent.Session{AgentName: "Claude Code"}
	v := renderTranscript(entries, 80, s, transcriptOpts{home: "/home/u"})
	out := ansi.Strip(v.content)
	for _, want := range []string{"⋯ 6 commands · ✎ 3 files +5 −1 · ✗ 1 failed · Bash, Edit, apply_patch", "☰ plan", "# Plan", "Step one."} {
		if !strings.Contains(out, want) {
			t.Errorf("folded: missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tasks") || v.stats.tools != 7 {
		t.Errorf("task list should fold, every call should count (tools = %d):\n%s", v.stats.tools, out)
	}

	out = ansi.Strip(renderTranscript(entries, 80, s, transcriptOpts{tools: true, home: "/home/u"}).content)
	for _, want := range []string{
		"$ go test ./... ✗",
		"✎ ~/p/a.go +2 −1",
		"✎ /p/a.go, /p/b.go +3",
		"· Read  /p/a.go",
		"⎇ Agent  Survey adapters",
		"☑ tasks  1/2 done", "◐ test",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("one per line: missing %q:\n%s", want, out)
		}
	}
}

func readerEntries() []agent.Entry {
	return []agent.Entry{
		{Role: agent.RoleUser, Text: "first ask\nwith detail"},
		{Role: agent.RoleAssistant, Text: "Looking."},
		{Role: agent.RoleTool, Text: "Edit · a.go", Kind: agent.ToolEdit, Added: 1},
		{Role: agent.RoleAssistant, Text: "Answer one."},
		{Role: agent.RoleUser, Text: "second ask"},
		{Role: agent.RoleTool, Text: "Agent · survey", Kind: agent.ToolAgent, Failed: true},
		{Role: agent.RoleAssistant, Text: "Answer two."},
	}
}

func TestRenderTranscriptModes(t *testing.T) {
	s := agent.Session{AgentName: "Claude Code"}
	v := renderTranscript(readerEntries(), 80, s, transcriptOpts{})
	if len(v.folds) != 2 || len(v.outline) != 2 {
		t.Fatalf("folds = %v, outline = %+v", v.folds, v.outline)
	}
	if o := v.outline; o[0].text != "first ask" || !o[0].edits || o[0].failed || !o[1].agents || !o[1].failed {
		t.Errorf("outline = %+v", o)
	}
	lines := strings.Split(ansi.Strip(v.content), "\n")
	if !strings.Contains(lines[v.folds[1].line], "⋯") {
		t.Errorf("fold line %d is not the steps line: %q", v.folds[1].line, lines[v.folds[1].line])
	}

	// enter on one turn unfolds it alone
	out := ansi.Strip(renderTranscript(readerEntries(), 80, s, transcriptOpts{open: map[int]bool{1: true}}).content)
	if !strings.Contains(out, "Looking.") || strings.Count(out, "⋯") != 1 {
		t.Errorf("only the first turn should unfold:\n%s", out)
	}

	out = ansi.Strip(renderTranscript(readerEntries(), 80, s, transcriptOpts{mode: modeChat}).content)
	if strings.Contains(out, "⋯") || strings.Contains(out, "Looking.") || !strings.Contains(out, "Answer two.") {
		t.Errorf("conversation should be prompts and answers only:\n%s", out)
	}
	cv := renderTranscript(readerEntries(), 80, s, transcriptOpts{mode: modeActions})
	out = ansi.Strip(cv.content)
	if strings.Contains(out, "Answer") || !strings.Contains(out, "✎ a.go +1") || !strings.Contains(out, "⎇ Agent  survey ✗") {
		t.Errorf("actions should be the calls only:\n%s", out)
	}
	if len(cv.folds) != 0 || cv.stats != v.stats {
		t.Errorf("actions: folds %v, stats %+v vs %+v", cv.folds, cv.stats, v.stats)
	}
}

// The reader keys: m cycles the view, ] picks a turn, enter unfolds it; on a
// wide terminal the rail lists the prompts.
func TestReaderKeys(t *testing.T) {
	m := tableTab(t, 1, 160, 30)
	m.Update(transcriptMsg{title: "t", session: agent.Session{AgentName: "Claude Code"}, entries: readerEntries()})
	if m.mode != sessModeDoc {
		t.Fatal("reader not open")
	}
	if w, h := lipgloss.Width(m.View()), lipgloss.Height(m.View()); w > 160 || h > 30 {
		t.Errorf("reader %dx%d outside 160x30", w, h)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "1 first ask") || !strings.Contains(view, "2 second ask") {
		t.Errorf("rail missing:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.docSel != 0 || !strings.Contains(ansi.Strip(m.View()), "▶ ⋯") {
		t.Fatalf("] should pick the first turn (sel %d):\n%s", m.docSel, ansi.Strip(m.View()))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Looking.") || !m.docOpts.open[1] {
		t.Errorf("enter should unfold the picked turn:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if m.docOpts.mode != modeChat || m.docOpts.open != nil || !strings.Contains(ansi.Strip(m.View()), "conversation ·") {
		t.Errorf("m should switch to conversation and reset unfolding: %+v", m.docOpts)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if strings.Contains(ansi.Strip(m.View()), "1 first ask") {
		t.Error("no rail below 150 columns")
	}
}

// enter on a subagent card opens its transcript; esc returns to the parent
// where it was.
func TestReaderOpensSubagent(t *testing.T) {
	m := tableTab(t, 1, 120, 30)
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "survey"},
		{Role: agent.RoleTool, Text: "Agent · Survey adapters", Kind: agent.ToolAgent, Sub: "/s/sub.jsonl", Calls: 21},
		{Role: agent.RoleAssistant, Text: "Done."},
	}
	m.Update(transcriptMsg{title: "parent", session: agent.Session{AgentID: "a", AgentName: "A"}, entries: entries})
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"}) // the turn's fold
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"}) // the subagent card
	if view := ansi.Strip(m.View()); !strings.Contains(view, "▶ ⎇ Agent  Survey adapters · 21 calls") {
		t.Fatalf("card not picked:\n%s", view)
	}
	m.inFlight = true // no spinner tick to wait on: the load is the only command
	loaded := m.openFold()()
	if msg, ok := loaded.(transcriptMsg); !ok || !msg.sub || msg.session.Path != "/s/sub.jsonl" || msg.title != "↳ Survey adapters" {
		t.Fatalf("enter should load the subagent: %+v", loaded)
	}
	m.Update(loaded)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "↳ Survey adapters") || !strings.Contains(view, "#1 Task") {
		t.Fatalf("subagent transcript not shown:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != sessModeDoc || m.docTitle != "parent" || m.docSel != 1 || !m.docOpts.steps {
		t.Errorf("esc should return to the parent as it was: mode %v, title %q, sel %d", m.mode, m.docTitle, m.docSel)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != sessModeList {
		t.Error("esc on the parent should close the reader")
	}
}

// Another branch is picked and opened like a subagent, but it is not a task.
func TestReaderOpensBranch(t *testing.T) {
	m := tableTab(t, 1, 120, 30)
	entries := []agent.Entry{
		{Role: agent.RoleUser, Text: "hi"},
		{Role: agent.RoleEvent, Text: "branch 2 of 2"},
		{Role: agent.RoleEvent, Text: "other branch · first try", Sub: "/s/p.jsonl#leaf1"},
		{Role: agent.RoleUser, Text: "try another way"},
	}
	m.Update(transcriptMsg{title: "parent", session: agent.Session{AgentID: "a", AgentName: "A"}, entries: entries})
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "▶─ other branch · first try ──") {
		t.Fatalf("branch not picked:\n%s", view)
	}
	m.inFlight = true
	msg, ok := m.openFold()().(transcriptMsg)
	if !ok || msg.task || msg.session.Path != "/s/p.jsonl#leaf1" || msg.title != "↳ first try" {
		t.Fatalf("enter should load the branch: %+v", msg)
	}
	m.Update(msg)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "#1 You") {
		t.Errorf("a branch's prompts are the user's:\n%s", view)
	}
}
