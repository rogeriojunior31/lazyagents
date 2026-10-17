package recovery

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

func run(t *testing.T, svc *Service, args ...string) (string, string, int) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := cli.Run(append([]string{"config-backups"}, args...), cli.Context{Out: &out, Err: &stderr}, commands(svc))
	return out.String(), stderr.String(), code
}

func TestCLIRequiresReviewedPreviewAndRechecksChanges(t *testing.T) {
	svc, _, path, id := fixture(t)
	out, stderr, code := run(t, svc, "list", "--query", "claude-code provider.apply settings.json", "--json")
	var entries []Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil || code != 0 || len(entries) != 1 {
		t.Fatalf("list: %s %s %d %v", out, stderr, code, err)
	}
	out, stderr, code = run(t, svc, "preview", id, "--json")
	var preview Preview
	if err := json.Unmarshal([]byte(out), &preview); err != nil || code != 0 || strings.Contains(out, "provider-secret") || preview.Review == "" {
		t.Fatalf("preview: %s %s %d %v", out, stderr, code, err)
	}
	if _, _, code := run(t, svc, "restore", id); code != 1 {
		t.Fatal("unreviewed restore accepted")
	}
	write(t, path, []byte(`{"foreign":"new edit"}`))
	if _, stderr, code := run(t, svc, "restore", id, "--review", preview.Review); code != 1 || !strings.Contains(stderr, "stale") {
		t.Fatalf("stale review accepted: %s", stderr)
	}
	out, _, _ = run(t, svc, "preview", id, "--json")
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := run(t, svc, "restore", id, "--review", preview.Review); code != 0 {
		t.Fatal(stderr)
	}
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "original-private") {
		t.Fatal("reviewed CLI restore failed")
	}
	for _, args := range [][]string{{"preview"}, {"preview", id, "extra"}, {"restore", id, "--yes"}, {"list", "extra"}, {"unknown"}} {
		if _, _, code := run(t, svc, args...); code != 1 {
			t.Fatalf("accepted invalid command: %v", args)
		}
	}
}
