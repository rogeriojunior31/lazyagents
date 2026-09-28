// Package plugintest builds the fake plugin (testdata/fakeplugin) that tests of
// plugins and app run on every OS. Only tests import it.
package plugintest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	once  sync.Once
	built string
	fail  string
	noGo  bool
)

// Build compiles the fake plugin once per test binary and returns its path;
// without a Go toolchain the test is skipped.
func Build(t *testing.T) string {
	t.Helper()
	once.Do(func() {
		_, self, _, _ := runtime.Caller(0)
		src := filepath.Join(filepath.Dir(self), "..", "testdata", "fakeplugin")
		gobin, err := exec.LookPath("go")
		if err != nil {
			gobin = filepath.Join(runtime.GOROOT(), "bin", "go"+exe())
		}
		if _, err := os.Stat(gobin); err != nil {
			noGo, fail = true, "no Go toolchain to build the fake plugin"
			return
		}
		dir, err := os.MkdirTemp("", "fakeplugin-")
		if err != nil {
			fail = err.Error()
			return
		}
		out := filepath.Join(dir, "fakeplugin"+exe())
		cmd := exec.Command(gobin, "build", "-o", out, ".")
		cmd.Dir = src
		if msg, err := cmd.CombinedOutput(); err != nil {
			fail = "building the fake plugin: " + err.Error() + "\n" + string(msg)
			return
		}
		built = out
	})
	switch {
	case noGo:
		t.Skip(fail)
	case built == "":
		t.Fatal(fail)
	}
	return built
}

// Install copies the fake plugin into dir as plugin id (id.exe on Windows,
// which is how plugins are discovered there) and returns its path.
func Install(t *testing.T, dir, id string) string {
	t.Helper()
	data, err := os.ReadFile(Build(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+exe())
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func exe() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
