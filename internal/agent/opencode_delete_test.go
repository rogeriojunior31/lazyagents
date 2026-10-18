package agent

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeOpenCode writes an opencode stand-in: `export <id>` prints exportOut
// (exit exportCode) and `session delete <id>` leaves a marker in dir.
func fakeOpenCode(t *testing.T, exportOut string, exportCode int) (*OpenCode, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = export ]; then echo 'Exporting session' >&2; printf '%s' '" + exportOut + "'; exit " + string(rune('0'+exportCode)) + "; fi\n" +
		"if [ \"$1\" = session ] && [ \"$2\" = delete ]; then touch '" + filepath.Join(dir, "deleted") + "'; fi\n"
	bin := filepath.Join(dir, "opencode")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	o := NewOpenCode(t.TempDir())
	o.Look = func(string) (string, error) { return bin, nil }
	return o, filepath.Join(dir, "deleted")
}

func TestOpenCodeDeleteBacksUpFirst(t *testing.T) {
	o, deleted := fakeOpenCode(t, `{"info":{"id":"ses_1"},"messages":[]}`, 0)
	backups := t.TempDir()
	if err := o.DeleteSession(Session{ID: "ses_1"}, backups); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(deleted); err != nil {
		t.Error("session delete did not run")
	}
	files, _ := filepath.Glob(filepath.Join(backups, "opencode-ses_1.*.json"))
	if len(files) != 1 {
		t.Fatalf("backups = %v, want one export", files)
	}
	info, _ := os.Stat(files[0])
	data, _ := os.ReadFile(files[0])
	if info.Mode().Perm() != 0o600 || !strings.Contains(string(data), `"ses_1"`) {
		t.Errorf("backup mode %v, content %q", info.Mode().Perm(), data)
	}
}

// Without a usable export nothing is deleted.
func TestOpenCodeDeleteRefusesWithoutBackup(t *testing.T) {
	cases := map[string]struct {
		out  string
		code int
		id   string
	}{
		"export fails":     {out: "", code: 1, id: "ses_1"},
		"export not JSON":  {out: "Session not found", code: 0, id: "ses_1"},
		"unsafe id":        {out: `{}`, code: 0, id: "../ses_1"},
		"empty JSON":       {out: `{}`, id: "ses_1"},
		"wrong ID":         {out: `{"info":{"id":"other"},"messages":[]}`, id: "ses_1"},
		"missing messages": {out: `{"info":{"id":"ses_1"}}`, id: "ses_1"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			o, deleted := fakeOpenCode(t, c.out, c.code)
			backups := t.TempDir()
			if err := o.DeleteSession(Session{ID: c.id}, backups); err == nil {
				t.Fatal("want an error")
			}
			if _, err := os.Stat(deleted); err == nil {
				t.Error("session was deleted without a backup")
			}
			if files, _ := os.ReadDir(backups); len(files) != 0 {
				t.Errorf("unexpected backups: %v", files)
			}
		})
	}
}

func TestOpenCodeV2BacksUpDescendantsBeforeDelete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in")
	}
	for _, failChild := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "child export fails"}[failChild], func(t *testing.T) {
			home := t.TempDir()
			if err := copyFixtureHome(filepath.Join(fixturesDir, "opencode", "2.0.32", "session-links", "home"), home); err != nil {
				if errors.Is(err, errNoSQLite) {
					t.Skip(err)
				}
				t.Fatal(err)
			}
			o := NewOpenCode(home)
			sessions, err := o.ListSessions()
			if err != nil || len(sessions) != 1 {
				t.Fatalf("%+v %v", sessions, err)
			}
			parent := sessions[0]
			dir := t.TempDir()
			deleted, bin := filepath.Join(dir, "deleted"), filepath.Join(dir, "opencode")
			script := "#!/bin/sh\nset -eu\ntest \"$1\" = session\ntest \"$4\" = --standalone\n" +
				"if [ \"$2\" = export ]; then\n"
			if failChild {
				script += "  test \"$3\" = '" + parent.ID + "' || exit 1\n"
			}
			script += "  printf '{\"info\":{\"id\":\"%s\"},\"messages\":[]}' \"$3\"\n" +
				"elif [ \"$2\" = delete ]; then touch '" + deleted + "'; else exit 1; fi\n"
			if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			o.Look = func(name string) (string, error) {
				if name == "opencode" {
					return bin, nil
				}
				return exec.LookPath(name)
			}
			backups := t.TempDir()
			err = o.DeleteSession(parent, backups)
			if (err != nil) != failChild {
				t.Fatalf("delete error=%v", err)
			}
			_, statErr := os.Stat(deleted)
			if failChild && !os.IsNotExist(statErr) || !failChild && statErr != nil {
				t.Fatalf("delete marker=%v", statErr)
			}
			files, _ := os.ReadDir(backups)
			want := 2
			if failChild {
				want = 1
			}
			if len(files) != want {
				t.Fatalf("backups=%d want=%d", len(files), want)
			}
			for _, f := range files {
				info, _ := f.Info()
				if info.Mode().Perm() != 0o600 {
					t.Fatal("export backup is not private")
				}
			}
		})
	}
}
