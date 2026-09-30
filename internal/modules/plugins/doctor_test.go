package plugins

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/modules/plugins/plugintest"
)

func doctorPlugin(t *testing.T, mode, exit string) (*Service, Plugin) {
	t.Helper()
	s := newGoService(t)
	plugintest.Install(t, s.Dir, "doc")
	t.Setenv("FAKEPLUGIN_MODE", mode)
	t.Setenv("FAKEPLUGIN_EXIT", exit)
	t.Setenv("FAKEPLUGIN_PIDFILE", filepath.Join(t.TempDir(), "child.pid"))
	pls, _ := s.List()
	return s, pls[0]
}

func TestRunDoctorExitCode(t *testing.T) {
	s, pl := doctorPlugin(t, "", "0")
	if ok, err := s.RunDoctor(pl, io.Discard); !ok || err != nil {
		t.Errorf("exit 0 = %v %v, want ok", ok, err)
	}
	s, pl = doctorPlugin(t, "", "3")
	if ok, err := s.RunDoctor(pl, io.Discard); ok || err != nil {
		t.Errorf("exit 3 = %v %v, want a reported problem", ok, err)
	}
}

// A doctor that hangs fails within the deadline instead of hanging doctor.
func TestRunDoctorDeadline(t *testing.T) {
	s, pl := doctorPlugin(t, "hang", "")
	s.Doctor = 1500 * time.Millisecond
	start := time.Now()
	ok, err := s.RunDoctor(pl, io.Discard)
	if ok || err == nil || !strings.Contains(err.Error(), "did not finish within") {
		t.Fatalf("hanging doctor = %v %v", ok, err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("the deadline took %s to stop the doctor", took)
	}
}

// The doctor's output reaches out even though it runs off the terminal.
func TestRunDoctorOutput(t *testing.T) {
	s, pl := doctorPlugin(t, "cli", "")
	var out strings.Builder
	if ok, err := s.RunDoctor(pl, &out); ok || err != nil || !strings.HasPrefix(out.String(), "doctor ") {
		t.Errorf("RunDoctor = %v %v, output %q", ok, err, out.String())
	}
}
