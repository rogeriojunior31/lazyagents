package recovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := fsutil.WriteAtomic(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (*Service, *agent.Claude, string, string) {
	t.Helper()
	home := t.TempDir()
	c := agent.NewClaude(home)
	svc := New([]agent.Adapter{c, agent.NewCodex(home), agent.NewPi(home), agent.NewCrush(home)}, core.PathsIn(home).BackupsDir())
	path := c.ProviderFile()
	write(t, path, []byte(`{"foreign":{"setting":"original-private"},"env":{"OTHER_TOKEN":"custom-secret"}}`))
	if err := c.ApplyProvider(agent.ProviderProfile{Name: "fixture", BaseURL: "https://example.test/v1", Token: "provider-secret", Model: "m"}, svc.dir); err != nil {
		t.Fatal(err)
	}
	entries, err := svc.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("snapshots=%v err=%v", entries, err)
	}
	return svc, c, path, entries[0].ID
}

func TestRecoveryPreviewsAndRestoresReviewedWholeFile(t *testing.T) {
	svc, c, path, id := fixture(t)
	if err := c.AddHook(agent.Hook{Event: agent.HookStop, Command: "echo hook-secret"}, svc.dir); err != nil {
		t.Fatal(err)
	}
	if err := c.ClearProvider(svc.dir); err != nil {
		t.Fatal(err)
	}
	current := []byte(`{"foreign":{"setting":"edited-private"},"env":{"NEW_TOKEN":"current-secret"},"hooks":{"Stop":[]}}`)
	write(t, path, current)
	p, err := svc.Preview(id)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := json.Marshal(p)
	for _, secret := range []string{"provider-secret", "custom-secret", "current-secret", "original-private", "edited-private", "hook-secret"} {
		if strings.Contains(string(text), secret) {
			t.Fatalf("preview leaked %s", secret)
		}
	}
	if !strings.Contains(strings.Join(p.Changes, "\n"), "/foreign/setting") {
		t.Fatal("unrelated edit omitted from diff")
	}
	if err := svc.Restore(id, ""); err == nil {
		t.Fatal("restored without a reviewed preview")
	}
	if err := svc.Restore(id, p.Review); err != nil {
		t.Fatal(err)
	}
	archived, err := os.ReadFile(filepath.Join(svc.dir, id))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(archived) {
		t.Fatalf("restore: %s, %v", got, err)
	}
	entries, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	var safety string
	for _, entry := range entries {
		if entry.Operation == "config.restore" {
			safety = entry.ID
		}
	}
	if data, err := os.ReadFile(filepath.Join(svc.dir, safety)); safety == "" || err != nil || string(data) != string(current) {
		t.Fatal("current state was not backed up before replacement")
	}
}

func TestRecoveryRefusesStaleOrUnexpectedFiles(t *testing.T) {
	for _, change := range []string{"destination", "source", "metadata", "permissions", "destination-symlink", "source-symlink", "parent-symlink", "unapproved-target", "unapproved-agent"} {
		t.Run(change, func(t *testing.T) {
			if runtime.GOOS == "windows" && (strings.Contains(change, "symlink") || change == "permissions") {
				t.Skip("Unix permissions/symlinks")
			}
			svc, _, path, id := fixture(t)
			p, err := svc.Preview(id)
			if err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside")
			write(t, outside, []byte(`{"external":"keep"}`))
			switch change {
			case "destination":
				write(t, path, []byte(`{"concurrent":"keep"}`))
			case "source":
				write(t, filepath.Join(svc.dir, id), []byte("corrupt archive"))
			case "permissions":
				if err := os.Chmod(path, 0o400); err != nil {
					t.Fatal(err)
				}
			case "destination-symlink", "source-symlink":
				link := path
				if change == "source-symlink" {
					link = filepath.Join(svc.dir, id)
				}
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, link); err != nil {
					t.Fatal(err)
				}
			case "parent-symlink":
				parent := filepath.Dir(path)
				if err := os.Rename(parent, parent+"-moved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(parent+"-moved", parent); err != nil {
					t.Fatal(err)
				}
			default:
				meta := fsutil.BackupMetadataPath(filepath.Join(svc.dir, id))
				data, err := os.ReadFile(meta)
				if err != nil {
					t.Fatal(err)
				}
				var record fsutil.BackupRecord
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				switch change {
				case "metadata":
					record.Operation = "provider.clear"
				case "unapproved-target":
					record.Destination = outside
				case "unapproved-agent":
					record.Agent = "codex"
				}
				data, _ = json.Marshal(record)
				write(t, meta, data)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Restore(id, p.Review); err == nil {
				t.Fatal("accepted stale or unexpected restore")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatal("failed restore changed current data")
			}
			if data, err := os.ReadFile(outside); err != nil || string(data) != `{"external":"keep"}` {
				t.Fatal("unexpected destination was touched")
			}
		})
	}
}

func TestRecoveryLegacyUnknownAndAgentCollisions(t *testing.T) {
	svc, c, path, _ := fixture(t)
	legacy, err := fsutil.Backup(path, svc.dir)
	if err != nil {
		t.Fatal(err)
	}
	pi := agent.NewPi(c.Home)
	piFile := pi.RecoveryFiles()[1].Path // adapter-approved settings file
	write(t, piFile, []byte(`{"defaultModel":"pi-model"}`))
	if _, err := fsutil.Backup(piFile, svc.dir, fsutil.BackupOrigin{Agent: pi.ID(), Operation: "provider.apply"}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(svc.dir, "skill.20261017T120000.tar.gz"), []byte("skill archive"))
	entries, err := svc.List()
	if err != nil || len(entries) != 3 {
		t.Fatalf("%v, %v", entries, err)
	}
	known := map[string]string{}
	for _, e := range entries {
		if e.ID == filepath.Base(legacy) {
			if e.Agent != "unknown" || e.Operation != "unknown" || e.Restorable || e.Destination != "" {
				t.Fatal("guessed legacy origin")
			}
		} else {
			known[e.Agent] = e.Destination
		}
	}
	if known[pi.ID()] != piFile || known[c.ID()] != path {
		t.Fatalf("same basename confused agents: %v", known)
	}
	if _, err := svc.Preview(filepath.Base(legacy)); err == nil {
		t.Fatal("legacy backup authorized for restoration")
	}
	for _, bad := range []string{"../" + filepath.Base(legacy), path, "missing"} {
		if _, err := svc.Preview(bad); err == nil {
			t.Fatalf("accepted invalid ID: %s", bad)
		}
	}
}

func TestRecoveryRecreatesMissingFileAndKeepsRestrictedMode(t *testing.T) {
	svc, _, path, id := fixture(t)
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	p, err := svc.Preview(id)
	if err != nil || p.CurrentExists {
		t.Fatalf("missing file preview: %+v %v", p, err)
	}
	if err := svc.Restore(id, p.Review); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	write(t, path, []byte(`{"changed":true}`))
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	p, err = svc.Preview(id)
	if err != nil || p.RestoredMode != "0400" {
		t.Fatalf("permissions: %+v %v", p, err)
	}
	if err := svc.Restore(id, p.Review); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Fatal("restore broadened access")
	}
}

func TestRecoveryCanRepairInvalidCurrentJSON(t *testing.T) {
	svc, _, path, id := fixture(t)
	write(t, path, []byte(`{"token":"private-unclosed`))
	p, err := svc.Preview(id)
	if err != nil || strings.Contains(strings.Join(p.Changes, "\n"), "private-unclosed") {
		t.Fatalf("corrupt current file preview failed: %v", err)
	}
	if err := svc.Restore(id, p.Review); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		t.Fatal("invalid current config was not repaired")
	}
}
