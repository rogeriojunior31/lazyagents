package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFailedVersionProbeIsValueSafe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	bin := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'private-probe-sentinel' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	c := NewCodex(t.TempDir())
	c.Look = func(string) (string, error) { return bin, nil }
	p := NewPi(t.TempDir())
	p.Look = c.Look
	for _, ad := range []Adapter{c, p} {
		a := ad.Detect()
		data, _ := json.Marshal(a)
		if a.DetectionError == "" || strings.Contains(string(data), "private-probe-sentinel") {
			t.Fatalf("unsafe or missing failure: %s", data)
		}
	}
}
