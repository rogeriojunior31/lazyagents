package plugins

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins/plugintest"
)

// newService is for the tests that still use shell fixtures (start failures,
// discovery by exec bit); the rest run the fake plugin through newGoService.
func newService(t *testing.T) *Service {
	t.Helper()
	// Windows runners have sh (Git Bash), but cannot exec a #! script directly.
	posixOnly(t)
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh in PATH")
	}
	return newGoService(t)
}

// newGoService runs on every OS: its plugins are the Go fake (plugintest).
func newGoService(t *testing.T) *Service {
	t.Helper()
	s := New(core.PathsIn(t.TempDir()))
	s.Handshake = 500 * time.Millisecond
	t.Cleanup(s.Close)
	return s
}

func writeFixture(t *testing.T, dir, name, script string) Plugin {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return Plugin{ID: strings.TrimSuffix(name, filepath.Ext(name)), Path: path}
}

func next(t *testing.T, p *Proc) (Msg, bool) {
	t.Helper()
	select {
	case m, ok := <-p.Events:
		return m, ok
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not answer")
		return Msg{}, false
	}
}

func TestStartSendClose(t *testing.T) {
	t.Setenv("FAKEPLUGIN_MODE", "good")
	s := newGoService(t)
	pl := Plugin{ID: "good", Path: plugintest.Install(t, s.Dir, "good")}
	p, err := s.Start(pl, Msg{Width: 80, Height: 24})
	if err != nil {
		t.Fatal(err)
	}
	if p.Manifest.Title != "Hello" || len(p.Manifest.Commands) != 1 || p.Manifest.Commands[0].Name != "ping" || len(p.Manifest.Help) != 1 {
		t.Errorf("manifest not sanitized: %+v", p.Manifest)
	}
	if m, _ := next(t, p); m.Type != "frame" || m.View != "hello" || m.Count == nil || *m.Count != 2 {
		t.Errorf("initial frame = %+v", m)
	}
	if err := p.Send(Msg{Type: "key", Key: "x"}); err != nil {
		t.Fatal(err)
	}
	if m, _ := next(t, p); m.View != "key x" {
		t.Errorf("echo = %+v", m)
	}
	_ = p.Send(Msg{Type: "reload"})
	if m, _ := next(t, p); m.View != "reloaded" {
		t.Errorf("reload = %+v", m)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := next(t, p); ok {
		t.Error("Events should close after Close")
	}
	if p.Err() == nil {
		t.Error("Err should explain the end")
	}
	if !strings.Contains(p.StderrTail(), "reload!") {
		t.Errorf("stderr not captured: %q", p.StderrTail())
	}
	if err := p.Send(Msg{Type: "key"}); err == nil {
		t.Error("Send after Close should fail")
	}
}

func TestStartFailures(t *testing.T) {
	s := newService(t)
	cases := map[string]string{
		"timeout":  "#!/bin/sh\nsleep 10\n",
		"garbage":  "#!/bin/sh\necho junk\nsleep 10\n",
		"notfirst": "#!/bin/sh\necho '{\"type\":\"frame\",\"view\":\"x\"}'\nsleep 10\n",
		"exit":     "#!/bin/sh\nexit 3\n",
		"huge":     "#!/bin/sh\nhead -c 2000000 /dev/zero | tr '\\0' a; echo\nsleep 10\n",
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			pl := writeFixture(t, s.Dir, name, script)
			start := time.Now()
			if _, err := s.Start(pl, Msg{}); err == nil {
				t.Fatal("Start should fail")
			} else if name == "exit" && !strings.Contains(err.Error(), "exit status 3") {
				t.Errorf("error without exit status: %v", err)
			}
			if d := time.Since(start); d > 5*time.Second {
				t.Errorf("Start took %s: the process was not stopped", d)
			}
		})
	}
}

func TestRunPassThrough(t *testing.T) {
	t.Setenv("FAKEPLUGIN_MODE", "cli")
	s := newGoService(t)
	pl := Plugin{ID: "cli", Path: plugintest.Install(t, s.Dir, "cli")}
	var out bytes.Buffer
	if code := s.Run(pl, []string{"a"}, nil, &out, &out); code != 7 {
		t.Errorf("exit = %d, want 7", code)
	}
	if want := "a " + s.paths.DataDir; !strings.Contains(out.String(), want) {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if code := s.Run(Plugin{ID: "x", Path: "/nonexistent"}, nil, nil, &out, &out); code != 1 {
		t.Errorf("missing binary = %d, want 1", code)
	}
}

func TestList(t *testing.T) {
	posixOnly(t)
	s := newService(t)
	writeFixture(t, s.Dir, "hello", "#!/bin/sh\n")
	writeFixture(t, s.Dir, "hello.py", "#!/bin/sh\n") // duplicate id
	writeFixture(t, s.Dir, "Bad Name", "#!/bin/sh\n")
	if err := os.WriteFile(filepath.Join(s.Dir, "noexec"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(s.Dir, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	pls, warns := s.List()
	if len(pls) != 1 || pls[0].ID != "hello" {
		t.Errorf("List = %+v", pls)
	}
	if len(warns) != 3 {
		t.Errorf("warnings = %v", warns)
	}
	if pls, warns := New(core.PathsIn(t.TempDir())).List(); pls != nil || warns != nil {
		t.Error("a missing dir should list nothing")
	}
}

// posixOnly skips discovery by exec bit, which Windows does not have; its
// discovery (.exe) has TestListWindowsExe.
func posixOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake plugins are POSIX shell scripts")
	}
}

// On Windows a plugin is an .exe; other files are ignored with a notice.
func TestListWindowsExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows discovery")
	}
	s := newService(t)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tool.exe", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(s.Dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pls, warns := s.List()
	if len(pls) != 1 || pls[0].ID != "tool" || len(warns) != 1 {
		t.Errorf("List = %+v, warnings = %v", pls, warns)
	}
}

func TestCleanView(t *testing.T) {
	cases := map[string]string{
		"a\x1b[31mb\x1b[0m":      "a\x1b[31mb\x1b[0m",
		"\x1b[38;2;1;2;3mx":      "\x1b[38;2;1;2;3mx",
		"a\x1b[2Jb\x1b[Hc":       "abc",
		"a\x1b]8;;http://x\x07b": "ab",
		"a\x1b]0;t\x1b\\b":       "ab",
		"a\r\nb\tc\x07":          "a\nb    c",
		"line\x1b7\x1b8end":      "lineend",
		"trunc\x1b[3":            "trunc",
		"naïve ✓ 界":              "naïve ✓ 界",
	}
	for in, want := range cases {
		if got := CleanView(in); got != want {
			t.Errorf("CleanView(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestManifestStripsTerminalControls(t *testing.T) {
	raw := "\x1b]52;c;secret\a\x1b[31mhello\x1b[0m"
	m := cleanManifest("test", Msg{Title: raw, Help: []HelpGroup{{Title: raw, Keys: [][2]string{{raw, raw}}}}, Commands: []Command{{Name: "test", Desc: raw}}})
	for _, got := range []string{m.Title, m.Help[0].Title, m.Help[0].Keys[0][0], m.Commands[0].Desc} {
		if got != "hello" {
			t.Fatalf("unsafe metadata: %q", got)
		}
	}
}
