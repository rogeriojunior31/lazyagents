package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

func snapshotFiles(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return slices.DeleteFunc(entries, func(e os.DirEntry) bool { return e.IsDir() })
}

func TestConfigBackupsRecordAgentAndOperation(t *testing.T) {
	home := t.TempDir()
	for _, ad := range []Adapter{NewClaude(home), NewCodex(home), NewPi(home), NewCrush(home)} {
		t.Run(ad.ID(), func(t *testing.T) {
			backups := t.TempDir()
			for _, file := range ad.(ConfigRecoveryHost).RecoveryFiles() {
				content := "{}"
				if filepath.Ext(file.Path) != ".json" {
					content = "# user config\n"
				}
				writeFile(t, file.Path, content)
			}
			host := ad.(ProviderHost)
			profile := ProviderProfile{Name: "fixture", BaseURL: "https://example.test/v1", Model: "m", EnvKey: "TEST_KEY", WireAPI: "responses"}
			if ad.ID() == "crush" {
				profile.WireAPI = "chat"
			}
			if err := host.ApplyProvider(profile, backups); err != nil {
				t.Fatal(err)
			}
			if err := host.ClearProvider(backups); err != nil {
				t.Fatal(err)
			}
			if host, ok := ad.(HooksHost); ok {
				one := Hook{Event: HookPreToolUse, Command: "echo fixture"}
				if err := host.AddHook(one, backups); err != nil {
					t.Fatal(err)
				}
				if err := host.RemoveHook(one, backups); err != nil {
					t.Fatal(err)
				}
			}
			operations := map[string]bool{}
			for _, file := range snapshotFiles(t, backups) {
				data, err := os.ReadFile(fsutil.BackupMetadataPath(filepath.Join(backups, file.Name())))
				if err != nil {
					t.Fatal(err)
				}
				var record fsutil.BackupRecord
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				if record.Agent != ad.ID() || !filepath.IsAbs(record.Destination) {
					t.Fatalf("wrong origin: %+v", record)
				}
				operations[record.Operation] = true
			}
			for _, op := range []string{"provider.apply", "provider.clear"} {
				if !operations[op] {
					t.Errorf("missing %s snapshot", op)
				}
			}
			if _, ok := ad.(HooksHost); ok {
				for _, op := range []string{"hook.install", "hook.uninstall"} {
					if !operations[op] {
						t.Errorf("missing %s snapshot", op)
					}
				}
			}
		})
	}
}

func TestRecoveryPreviewsHideEveryValue(t *testing.T) {
	before := []byte(`{"env":{"KEY":"old-secret"},"unknown":{"password":"private-before"},"large":9007199254740992}`)
	after := []byte(`{"env":{"KEY":"new-secret"},"unknown":{"password":"private-after"},"large":9007199254740993}`)
	changes, err := recoveryJSONChanges(before, after)
	if err != nil || len(changes) != 3 {
		t.Fatalf("%v, %v", changes, err)
	}
	text := strings.Join(changes, "\n")
	if strings.Contains(text, "secret") || strings.Contains(text, "private-") || !strings.Contains(text, "/unknown/password") {
		t.Fatal(text)
	}
	lines := strings.Join(recoveryLineChanges([]byte("# secret comment\nkey = 'old-secret'\n"), []byte("# new secret\nkey = 'new-secret'\n")), "\n")
	if strings.Contains(lines, "secret") || !strings.Contains(lines, "key") {
		t.Fatal(lines)
	}
}

func TestRecoveryDoesNotRestoreCodexApprovals(t *testing.T) {
	for _, text := range []string{"[hooks.state.x]\ntrusted_hash = 'accepted'\n", "[features]\nhooks = true\n", `features = { hooks = true }`, `"\u0068ooks" = { state = {} }`} {
		if _, err := recoveryCodexChanges([]byte(text), []byte("model = 'gpt-5'\n")); err == nil {
			t.Errorf("approval restore accepted: %s", text)
		}
	}
}
