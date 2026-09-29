package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPiDetect(t *testing.T) {
	home := t.TempDir()
	if a := (&Pi{Home: home, Look: noBin}).Detect(); a.Installed || a.SupportsSkills() {
		t.Errorf("pi should not be detected in an empty home: %+v", a)
	}

	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := (&Pi{Home: home, Look: noBin}).Detect()
	if !a.Installed || a.ID != "pi" || a.Short != "P" {
		t.Fatalf("pi with ~/.pi/agent: %+v", a)
	}
	if want := filepath.Join(agentDir, "skills"); a.ManagedDir != want || a.ReadDirs[0] != want {
		t.Errorf("ManagedDir = %s, ReadDirs = %v, want %s first", a.ManagedDir, a.ReadDirs, want)
	}
	if want := filepath.Join(home, ".agents", "skills"); len(a.ReadDirs) != 2 || a.ReadDirs[1] != want || a.SharedDir != want {
		t.Errorf("ReadDirs = %v, want %s second", a.ReadDirs, want)
	}

	// PI_CODING_AGENT_DIR, with ~ expanded against the injected home
	a = (&Pi{Home: home, Dir: "~/custom", Look: noBin}).Detect()
	if a.Installed {
		t.Errorf("a missing PI_CODING_AGENT_DIR should not count: %+v", a)
	}
	if err := os.MkdirAll(filepath.Join(home, "custom"), 0o755); err != nil {
		t.Fatal(err)
	}
	a = (&Pi{Home: home, Dir: "~/custom", Look: noBin}).Detect()
	if want := filepath.Join(home, "custom", "skills"); a.ManagedDir != want {
		t.Errorf("ManagedDir with override = %s, want %s", a.ManagedDir, want)
	}
}

// `pi` is a generic name: a binary with no agent dir only counts when
// `pi --version` prints a bare version.
func TestPiForeignBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake binaries are shell scripts")
	}
	for _, tc := range []struct {
		out  string
		want bool
	}{
		{"0.87.1", true},
		{"pi 3.14 (some other tool)", false},
	} {
		bin := filepath.Join(t.TempDir(), "pi")
		writeFile(t, bin, "#!/bin/sh\necho '"+tc.out+"'\n")
		if err := os.Chmod(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		look := func(string) (string, error) { return bin, nil }
		a := (&Pi{Home: t.TempDir(), Look: look}).Detect()
		if a.Installed != tc.want {
			t.Errorf("--version %q: Installed = %v, want %v", tc.out, a.Installed, tc.want)
		}
		if tc.want && a.Version != tc.out {
			t.Errorf("Version = %q, want %q", a.Version, tc.out)
		}
	}
}

// piFixture is a session recorded by pi 0.87.1 against a local fake model: a
// named session, two turns, each a bash tool call then a reply.
func piFixture(t *testing.T, sessions string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "pi-session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessions, "--work-proj--", "2026-09-29T10-55-11-084Z_01a0eccd-e2ec-73e2-a927-de80165ad726.jsonl")
	writeFile(t, path, string(data))
	return path
}

func TestPiSessions(t *testing.T) {
	home := t.TempDir()
	p := &Pi{Home: home, Look: noBin}
	path := piFixture(t, filepath.Join(home, ".pi", "agent", "sessions"))

	list, err := p.ListSessions()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSessions = %v, %v", list, err)
	}
	s := list[0]
	if s.ID != "01a0eccd-e2ec-73e2-a927-de80165ad726" || s.CWD != "/work/proj" || s.Title != "Fixture session" || s.Path != path {
		t.Errorf("session = %+v", s)
	}
	argv, dir, ok := p.ResumeCmd(s)
	if !ok || strings.Join(argv, " ") != "pi --session "+path || dir != home {
		t.Errorf("ResumeCmd = %v %q %v (cwd missing: falls back to home)", argv, dir, ok)
	}

	// a later /name wins; the index reads only the appended line
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"session_info","id":"ffff0001","parentId":"291bec5b","timestamp":"2026-09-29T11:00:00.000Z","name":"Renamed"}` + "\n")
	f.Close()
	if list, _ = p.ListSessions(); list[0].Title != "Renamed" {
		t.Errorf("title after rename = %q", list[0].Title)
	}

	backups := t.TempDir()
	if err := p.DeleteSession(s, backups); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("session file still there")
	}
	if entries, _ := os.ReadDir(backups); len(entries) == 0 {
		t.Error("no backup written")
	}
	if list, _ = p.ListSessions(); len(list) != 0 {
		t.Errorf("deleted session still listed: %v", list)
	}
}

// Without a /name the first prompt is the title; the id is URI-decoded.
func TestPiSessionTitleAndID(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".pi", "agent", "sessions", "--work--")
	writeFile(t, filepath.Join(dir, "2026-09-29T10-00-00-000Z_my%20id.jsonl"),
		`{"type":"session","version":3,"id":"my id","timestamp":"2026-09-29T10:00:00.000Z","cwd":"/work"}
{"type":"message","id":"a1","parentId":null,"timestamp":"2026-09-29T10:00:01.000Z","message":{"role":"user","content":"fix  the\nbuild","timestamp":1}}
`)
	list, _ := (&Pi{Home: home, Look: noBin}).ListSessions()
	if len(list) != 1 || list[0].ID != "my id" || list[0].Title != "fix the build" {
		t.Fatalf("list = %+v", list)
	}
}

func TestPiTranscriptActiveBranch(t *testing.T) {
	home := t.TempDir()
	path := piFixture(t, t.TempDir())
	p := &Pi{Home: home, Look: noBin}
	got, err := p.Transcript(Session{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{RoleUser, "run echo fixture please"}, {RoleTool, "bash · echo fixture"}, {RoleAssistant, "The command printed fixture."},
		{RoleUser, "and once more"}, {RoleTool, "bash · echo fixture"}, {RoleAssistant, "The command printed fixture."},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("transcript =\n%v\nwant\n%v", got, want)
	}

	// a branch from the first reply (c4d16194) becomes the leaf: the second
	// turn is on the other branch and leaves the transcript
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"message","id":"bbbb0001","parentId":"c4d16194","timestamp":"2026-09-29T11:00:00.000Z","message":{"role":"user","content":[{"type":"text","text":"try another way"}],"timestamp":2}}` + "\n")
	f.Close()
	got, _ = p.Transcript(Session{Path: path})
	want = append(want[:3:3], Entry{RoleUser, "try another way"})
	if !slices.Equal(got, want) {
		t.Fatalf("branch transcript =\n%v\nwant\n%v", got, want)
	}
}

// PI_CODING_AGENT_SESSION_DIR beats sessionDir in settings.json, which beats
// the default; a relative sessionDir is per project, so the default is used.
func TestPiSessionsDir(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, ".pi", "agent", "settings.json")
	for _, tc := range []struct{ env, setting, want string }{
		{"", "", ".pi/agent/sessions"},
		{"", "~/from-settings", "from-settings"},
		{"", "relative/dir", ".pi/agent/sessions"},
		{"~/from-env", "~/from-settings", "from-env"},
	} {
		writeFile(t, settings, `{"theme":"dark","sessionDir":"`+tc.setting+`"}`)
		if got := (&Pi{Home: home, SessionDir: tc.env}).sessionsDir(); got != filepath.Join(home, tc.want) {
			t.Errorf("env %q, setting %q: %s, want ~/%s", tc.env, tc.setting, got, tc.want)
		}
	}
}

// Version 1 sessions (before the tree) have no id/parentId: read in order.
func TestPiTranscriptVersion1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.jsonl")
	writeFile(t, path, `{"type":"session","id":"old","timestamp":"2025-01-01T00:00:00.000Z","cwd":"/work"}
{"type":"message","timestamp":"2025-01-01T00:00:01.000Z","message":{"role":"user","content":"hello","timestamp":1}}
{"type":"message","timestamp":"2025-01-01T00:00:02.000Z","message":{"role":"assistant","content":[{"type":"text","text":"hi"}],"timestamp":2}}
`)
	got, err := (&Pi{Home: t.TempDir(), Look: noBin}).Transcript(Session{Path: path})
	if want := []Entry{{RoleUser, "hello"}, {RoleAssistant, "hi"}}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("transcript = %v, %v; want %v", got, err, want)
	}
}

// The totals match what pi's own footer showed for the fixture session
// (input 340, output 28, cache read 200); cost is pi's recorded figure.
func TestPiUsage(t *testing.T) {
	home := t.TempDir()
	p := &Pi{Home: home, Look: noBin}
	path := piFixture(t, filepath.Join(home, ".pi", "agent", "sessions"))
	s := Session{Path: path, ID: "x", CWD: "/work/proj"}

	u, ok := p.SessionUsage(s)
	if !ok || u.Input != 340 || u.Output != 28 || u.CacheRead != 200 || u.CacheWrite != 0 || u.Model != "fake-model" {
		t.Fatalf("SessionUsage = %+v, %v", u, ok)
	}
	if cost, ok := EstimateCost(u); !ok || cost < 0.000415 || cost > 0.000417 {
		t.Errorf("cost = %v %v, want pi's 0.000416", cost, ok)
	}

	// a compaction summary has usage but no model: the session's applies
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"compaction","id":"cccc0001","parentId":"291bec5b","timestamp":"2026-09-29T11:00:00.000Z","summary":"s","firstKeptEntryId":"291bec5b","tokensBefore":1000,"usage":{"input":1000,"output":100,"cacheRead":0,"cacheWrite":0,"totalTokens":1100,"cost":{"total":0.5}}}` + "\n")
	f.Close()
	events, err := p.UsageEvents(s)
	if err != nil || len(events) == 0 {
		t.Fatalf("UsageEvents = %v, %v", events, err)
	}
	var sum Usage
	n := 0
	for _, ev := range events {
		sum.Input += ev.Usage.Input
		sum.Output += ev.Usage.Output
		sum.Cost += ev.Usage.Cost
		n += ev.N
		if ev.Model != "fake-model" || ev.CWD != "/work/proj" {
			t.Errorf("event %+v", ev)
		}
	}
	if sum.Input != 1340 || sum.Output != 128 || n != 5 || sum.Cost < 0.5004 || sum.Cost > 0.5005 {
		t.Errorf("events sum = %+v over %d responses", sum, n)
	}
	if last := events[len(events)-1]; !last.Time.Equal(time.Date(2026, 9, 29, 11, 0, 0, 0, time.UTC)) {
		t.Errorf("last event time = %v", last.Time)
	}
}

// AuthMode is API key when any provider is billed per token; only credential
// types are read, never values.
func TestPiAuthMode(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".pi", "agent")
	p := &Pi{Home: home, Look: noBin}
	if mode, _ := p.AuthMode(); mode != AuthUnknown {
		t.Errorf("no credentials: %v", mode)
	}
	writeFile(t, filepath.Join(dir, "settings.json"), `{"defaultProvider":"openai-codex"}`)
	writeFile(t, filepath.Join(dir, "auth.json"), `{"openai-codex":{"type":"oauth","access":"x","refresh":"y","expires":1}}`)
	if mode, detail := p.AuthMode(); mode != AuthSubscription || detail != "openai-codex" {
		t.Errorf("oauth only: %v %q", mode, detail)
	}
	writeFile(t, filepath.Join(dir, "models.json"), `{"providers":{"local":{"baseUrl":"http://x","apiKey":"$KEY"}}}`)
	if mode, _ := p.AuthMode(); mode != AuthAPIKey {
		t.Errorf("oauth plus a models.json key: %v", mode)
	}
	if got := p.credentialTypes(); len(got) != 2 || got["local"] != "api_key" || got["openai-codex"] != "oauth" {
		t.Errorf("credentialTypes = %v", got)
	}
}

// Calls to a provider signed in with OAuth are covered by the subscription:
// tokens count, cost is 0; calls billed per token keep pi's figure.
func TestPiUsageCoveredBySubscription(t *testing.T) {
	home := t.TempDir()
	p := &Pi{Home: home, Look: noBin}
	path := piFixture(t, filepath.Join(home, ".pi", "agent", "sessions"))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"message","id":"dddd0001","parentId":"291bec5b","timestamp":"2026-09-29T11:00:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":"x"}],"provider":"sub","model":"gpt-x","usage":{"input":10,"output":1,"cacheRead":0,"cacheWrite":0,"cost":{"total":9}},"timestamp":3}}` + "\n")
	f.Close()
	writeFile(t, filepath.Join(home, ".pi", "agent", "auth.json"), `{"sub":{"type":"oauth"},"fake":{"type":"api_key","key":"k"}}`)
	s := Session{Path: path, ID: "x"}
	events, _ := p.UsageEvents(s)
	var paid, covered float64
	for _, ev := range events {
		cost, ok := EstimateCost(ev.Usage)
		if !ok {
			t.Fatalf("event not priced: %+v", ev)
		}
		if ev.Model == "gpt-x" {
			covered += cost
			if !ev.Usage.Covered {
				t.Errorf("subscription call not covered: %+v", ev)
			}
		} else {
			paid += cost
		}
	}
	if covered != 0 || paid < 0.000415 || paid > 0.000417 {
		t.Errorf("covered = %v, paid = %v", covered, paid)
	}
	u, _ := p.SessionUsage(s)
	if u.Input != 350 || u.Covered || u.Cost < 0.000415 || u.Cost > 0.000417 {
		t.Errorf("SessionUsage = %+v", u)
	}
}
