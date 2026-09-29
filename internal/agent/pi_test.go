package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
	if want := filepath.Join(home, ".agents", "skills"); len(a.ReadDirs) != 2 || a.ReadDirs[1] != want {
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
