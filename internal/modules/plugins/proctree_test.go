package plugins

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins/plugintest"
)

// childPID waits for the fake plugin to report the child it started.
func childPID(t *testing.T, file string) int {
	t.Helper()
	for range 200 {
		if data, err := os.ReadFile(file); err == nil && len(data) > 0 {
			pid, _ := strconv.Atoi(string(data))
			return pid
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the plugin never reported its child")
	return 0
}

// mustBeGone fails when pid still runs, and kills it so no test leaves one.
func mustBeGone(t *testing.T, pid int, what string) {
	t.Helper()
	if !gone(pid) {
		kill(pid)
		t.Fatal(what)
	}
}

// Closing a plugin stops the processes it started, not only the plugin.
func TestCloseStopsProcessTree(t *testing.T) {
	s := newGoService(t)
	s.Handshake = 5 * time.Second // a fresh binary can be slow to start
	plugintest.Install(t, s.Dir, "spawner")
	pidfile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("FAKEPLUGIN_MODE", "spawn")
	t.Setenv("FAKEPLUGIN_PIDFILE", pidfile)
	pls, _ := s.List()
	p, err := s.Start(pls[0], Msg{})
	if err != nil {
		t.Fatal(err)
	}
	child := childPID(t, pidfile)
	_ = p.Close()
	mustBeGone(t, child, "the plugin's child outlived it")
}

// A doctor stopped by its deadline takes its children with it.
func TestRunDoctorDeadlineStopsTree(t *testing.T) {
	s, pl := doctorPlugin(t, "hang", "")
	s.Doctor = 3 * time.Second
	if _, err := s.RunDoctor(pl, io.Discard); err == nil {
		t.Fatal("want the deadline error")
	}
	mustBeGone(t, childPID(t, os.Getenv("FAKEPLUGIN_PIDFILE")), "the doctor's child outlived the deadline")
}

// A doctor that exits but leaves a child running: the child is stopped too
// (a process group on Unix, a Job Object on Windows).
func TestRunDoctorReapsOrphan(t *testing.T) {
	s, pl := doctorPlugin(t, "orphan", "")
	if ok, err := s.RunDoctor(pl, io.Discard); !ok || err != nil {
		t.Fatalf("RunDoctor = %v %v", ok, err)
	}
	mustBeGone(t, childPID(t, os.Getenv("FAKEPLUGIN_PIDFILE")), "the doctor's child outlived it")
}
