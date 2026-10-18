package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// errNoSQLite skips a fixture that needs the sqlite3 binary on a machine without it.
var errNoSQLite = errors.New("sqlite3 not installed")

var update = flag.Bool("update", false, "rewrite the fixture goldens and testdata/fixtures/README.md")

const fixturesDir = "testdata/fixtures"

// fixtureOrigin is origin.json, written by scripts/record-fixtures.go.
type fixtureOrigin struct {
	Agent    string `json:"agent"`
	Version  string `json:"version"`
	Recorded string `json:"recorded"`
	How      string `json:"how"`
	By       string `json:"by"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// fixtureGolden is what the adapter makes of a recorded session. Times that
// depend on when the test copies the files are left out.
type fixtureGolden struct {
	Sessions []goldenSession `json:"sessions"`
	Limits   *RateStatus     `json:"limits,omitempty"`
}

type goldenSession struct {
	ID         string    `json:"id"`
	CWD        string    `json:"cwd"`
	Title      string    `json:"title"`
	Resume     []string  `json:"resume,omitempty"`
	Transcript []Entry   `json:"transcript"`
	Usage      *Usage    `json:"usage,omitempty"`
	Events     *goldenEv `json:"events,omitempty"`
}

type goldenEv struct {
	Providers []string `json:"providers,omitempty"`
	Responses int      `json:"responses"`
	Usage     Usage    `json:"usage"`
	Models    []string `json:"models"`
}

// TestRecordedFixtures runs each adapter over the sessions its CLI recorded
// (testdata/fixtures/<agent>/<version>/home) and compares the result with the
// golden next to them: a CLI release that changes a private format shows up
// as a new fixture dir whose golden differs. Refresh with -update.
func TestRecordedFixtures(t *testing.T) {
	origins := fixtureOrigins(t)
	if len(origins) == 0 {
		t.Fatal("no fixtures: run go run scripts/record-fixtures.go")
	}
	for _, o := range origins {
		t.Run(o.Agent+"/"+o.Version, func(t *testing.T) {
			dir := filepath.Join(fixturesDir, o.Agent, o.Version)
			home := t.TempDir()
			if err := copyFixtureHome(filepath.Join(dir, "home"), home); err != nil {
				if errors.Is(err, errNoSQLite) {
					t.Skip(err)
				}
				t.Fatal(err) // a dump that does not load is a broken fixture
			}
			ad := ByID(All(home), o.Agent)
			if ad == nil {
				t.Fatalf("no adapter %q", o.Agent)
			}
			limits := serveLimits(t, dir, home, ad)
			before := fixtureConfigs(t, home)
			got := goldenFor(t, ad, limits)
			if after := fixtureConfigs(t, home); !reflect.DeepEqual(before, after) {
				t.Fatal("reading recorded sessions changed native configuration")
			}
			data, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, '\n')
			data = bytes.ReplaceAll(data, []byte(filepath.ToSlash(home)), []byte("/home/user"))
			golden := filepath.Join(dir, "golden.json")
			if *update {
				if err := os.WriteFile(golden, data, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run go test ./internal/agent -run TestRecordedFixtures -update)", err)
			}
			if !bytes.Equal(data, want) {
				t.Errorf("%s no longer matches what the adapter reads; if the change is intended, rerun with -update.\ngot:\n%s", golden, data)
			}
		})
	}
	readme := fixtureMatrix(origins)
	path := filepath.Join(fixturesDir, "README.md")
	if *update {
		if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if cur, _ := os.ReadFile(path); string(cur) != readme {
		t.Errorf("%s is stale: rerun with -update", path)
	}
}

func fixtureOrigins(t *testing.T) []fixtureOrigin {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(fixturesDir, "*", "*", "origin.json"))
	var out []fixtureOrigin
	for _, p := range paths {
		var o fixtureOrigin
		data, err := os.ReadFile(p)
		if err == nil {
			err = json.Unmarshal(data, &o)
		}
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if want := filepath.Join(fixturesDir, o.Agent, o.Version, "origin.json"); filepath.Clean(p) != want {
			t.Fatalf("%s names %s/%s", p, o.Agent, o.Version)
		}
		if o.Recorded == "" || o.How == "" || o.By == "" || (o.OS == "") != (o.Arch == "") {
			t.Fatalf("%s: incomplete origin metadata", p)
		}
		out = append(out, o)
	}
	return out
}

func fixtureConfigs(t *testing.T, home string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, path := range []string{
		".claude/settings.json", ".codex/config.toml", ".gemini/settings.json", ".gemini/projects.json",
		".config/opencode/opencode.json", ".config/crush/crush.json", ".config/crush/crushrc",
		".pi/agent/models.json", ".pi/agent/settings.json",
	} {
		data, err := os.ReadFile(filepath.Join(home, path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		out[path] = string(data)
	}
	return out
}

// Exercise writes only for existing ProviderHost implementations, using the
// same foreign configuration that the recorded CLI accepted.
func TestRecordedProviderPreservation(t *testing.T) {
	for _, o := range fixtureOrigins(t) {
		t.Run(o.Agent+"/"+o.Version, func(t *testing.T) {
			home := t.TempDir()
			ad := ByID(All(home), o.Agent)
			host, ok := ad.(ProviderHost)
			if !ok {
				t.Skip("no provider write capability")
			}
			// Config-only copy avoids a sqlite3 prerequisite for preservation tests.
			src := filepath.Join(fixturesDir, o.Agent, o.Version, "home")
			for path, data := range fixtureConfigs(t, src) {
				writeFile(t, filepath.Join(home, path), data)
			}
			before := fixtureConfigs(t, home)
			if len(before) == 0 {
				t.Skip("older session-only recording")
			}
			p := ProviderProfile{Name: "fixture", BaseURL: "http://127.0.0.1:18765/v1", Model: "fixture-model", EnvKey: "FIXTURE_KEY"}
			if o.Agent == "gemini-cli" {
				p.WireAPI, p.EnvKey = "gemini", "GEMINI_API_KEY"
			}
			backup := t.TempDir()
			if err := host.ApplyProvider(p, backup); err != nil {
				t.Fatal(err)
			}
			got, ok, err := host.ReadProvider()
			if err != nil || !ok || got.Model != p.Model || got.Token != "" {
				t.Fatalf("applied provider = %+v, %v, %v", got, ok, err)
			}
			if err := host.ClearProvider(backup); err != nil {
				t.Fatal(err)
			}
			if after := fixtureConfigs(t, home); !reflect.DeepEqual(canonicalFixtureConfigs(before), canonicalFixtureConfigs(after)) {
				t.Fatalf("provider round-trip changed foreign configuration:\nbefore: %v\nafter: %v", before, after)
			}
		})
	}
}

func canonicalFixtureConfigs(files map[string]string) map[string]string {
	out := map[string]string{}
	for path, data := range files {
		if strings.HasSuffix(path, ".json") {
			var v any
			if json.Unmarshal([]byte(data), &v) == nil {
				encoded, _ := json.Marshal(v)
				data = string(encoded)
			}
		}
		// Pi clear leaves an empty settings object if no defaults existed.
		if path == ".pi/agent/settings.json" && data == "{}" {
			continue
		}
		out[path] = data
	}
	return out
}

// copyFixtureHome copies a recorded home; an SQL dump becomes the database the
// CLI keeps (OpenCode), which needs the sqlite3 binary like the adapter does.
func copyFixtureHome(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// recordings name the throwaway home /home/user; paths in them must
		// point into this one (forward slashes stay valid on Windows and in JSON)
		data = bytes.ReplaceAll(data, []byte("/home/user"), []byte(filepath.ToSlash(dst)))
		if !strings.HasSuffix(path, ".sql") {
			return os.WriteFile(target, data, 0o644)
		}
		sqlite, err := exec.LookPath("sqlite3")
		if err != nil {
			return errNoSQLite
		}
		cmd := exec.Command(sqlite, strings.TrimSuffix(target, ".sql")+".db")
		cmd.Stdin = bytes.NewReader(data)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sqlite3 loading %s: %v: %s", rel, err, out)
		}
		return nil
	})
}

// serveLimits serves a recorded limits.json (Claude Code's usage endpoint) to
// the adapter, with a made-up login; it reports whether limits can be read.
func serveLimits(t *testing.T, dir, home string, ad Adapter) bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "limits.json"))
	c, isClaude := ad.(*Claude)
	if err != nil || !isClaude {
		return ad.ID() != "claude-code" // Claude limits come only from the network
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	c.UsageURL = srv.URL
	creds := `{"claudeAiOauth":{"accessToken":"fixture-token","expiresAt":4102444800000,"subscriptionType":"max"}}`
	if err := os.WriteFile(filepath.Join(c.configDir(), ".credentials.json"), []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}
	return true
}

func goldenFor(t *testing.T, ad Adapter, limits bool) fixtureGolden {
	t.Helper()
	sessions, err := ad.ListSessions()
	if err != nil || len(sessions) == 0 {
		t.Fatalf("ListSessions = %d sessions, %v", len(sessions), err)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	var g fixtureGolden
	for _, s := range sessions {
		gs := goldenSession{ID: s.ID, CWD: s.CWD, Title: s.Title}
		if argv, _, ok := ad.ResumeCmd(s); ok {
			gs.Resume = slices.Clone(argv)
			for i, a := range gs.Resume {
				if a == s.Path { // pi resumes by file, which lives in the test's temp home
					gs.Resume[i] = "<session file>"
				}
			}
		}
		if gs.Transcript, err = ad.Transcript(s); err != nil {
			t.Fatalf("Transcript(%s): %v", s.ID, err)
		}
		if ur, ok := ad.(UsageReader); ok {
			if u, ok := ur.SessionUsage(s); ok {
				// SQLite versions serialize the last floating-point digits differently.
				u.Cost = math.Round(u.Cost*1e12) / 1e12
				gs.Usage = &u
			}
		}
		if er, ok := ad.(UsageEventReader); ok {
			events, err := er.UsageEvents(s)
			if err != nil {
				t.Fatalf("UsageEvents(%s): %v", s.ID, err)
			}
			if len(events) > 0 {
				ev := &goldenEv{Usage: Usage{CostKnown: true}}
				for _, e := range events {
					ev.Responses += max(e.N, 1)
					ev.Usage.Input += e.Usage.Input
					ev.Usage.Output += e.Usage.Output
					ev.Usage.CacheRead += e.Usage.CacheRead
					ev.Usage.CacheWrite += e.Usage.CacheWrite
					ev.Usage.CacheWrite1h += e.Usage.CacheWrite1h
					if e.Usage.Tier != "" && !strings.Contains(ev.Usage.Tier, e.Usage.Tier) {
						ev.Usage.Tier = strings.Trim(ev.Usage.Tier+","+e.Usage.Tier, ",")
					}
					ev.Usage.Cost += e.Usage.Cost
					ev.Usage.CostKnown = ev.Usage.CostKnown && e.Usage.CostKnown
					ev.Usage.Incomplete = ev.Usage.Incomplete || e.Usage.Incomplete
					if e.Provider != "" && !slices.Contains(ev.Providers, e.Provider) {
						ev.Providers = append(ev.Providers, e.Provider)
					}
					if !slices.Contains(ev.Models, e.Model) {
						ev.Models = append(ev.Models, e.Model)
					}
				}
				ev.Usage.Cost = math.Round(ev.Usage.Cost*1e12) / 1e12
				gs.Events = ev
			}
		}
		g.Sessions = append(g.Sessions, gs)
	}
	if rl, ok := ad.(RateLimitReader); ok && limits {
		if st, err := rl.RateLimits(context.Background()); err == nil {
			st.FetchedAt = st.FetchedAt.UTC() // the golden must not depend on the machine's zone
			if st.Source == "api" {
				st.FetchedAt = time.Time{} // the moment of the call, not part of the format
			}
			for i := range st.Windows {
				st.Windows[i].ResetsAt = st.Windows[i].ResetsAt.UTC()
			}
			g.Limits = &st
		}
	}
	return g
}

// fixtureMatrix is testdata/fixtures/README.md: which CLI versions are pinned
// and where each fixture came from.
func fixtureMatrix(origins []fixtureOrigin) string {
	var b strings.Builder
	b.WriteString("<!-- Generated by `go test ./internal/agent -run TestRecordedFixtures -update`. Do not edit by hand. -->\n\n")
	b.WriteString("# Recorded CLI fixtures\n\n")
	b.WriteString("Sessions written by the real agent CLIs, one dir per version, read by `TestRecordedFixtures`. ")
	b.WriteString("`scripts/record-fixtures.go` records them in a throwaway home against a local fake model: no account or paid inference; updates and telemetry disabled, ")
	b.WriteString("paths replaced by `/work/proj` and `/home/user`, long strings (system prompts) cut, and a recording that still names the machine fails. ")
	b.WriteString("Limits are pinned from a real response with every value replaced: `-codex-limits-from` (a Codex rollout) and `-claude-limits` (one call to Claude Code's usage endpoint with your login, keeping only the fields lazyagents reads).\n\n")
	b.WriteString("Collected native settings are recorder inputs accepted by the CLI, not evidence of its defaults. OS/architecture missing from older recordings remain unknown. ")
	b.WriteString("Resume arguments in goldens do not prove native resumption or deletion. Gemini tool-call rendering and session timestamps remain M27.5 work; Hermes has no runnable CLI recording yet (F05 in BACKLOG.md).\n\n")
	b.WriteString("Gemini 0.64.0 usage goldens count durable response IDs once across repeated snapshots. OpenCode 2.0.32 usage goldens retain response provider/model, separate cache counters and authoritative recorded cost, including zero. Token/cost values in these local fake-model recordings are synthetic, not production billing.\n\n")
	b.WriteString("To pin a new CLI release, install it and run `go run scripts/record-fixtures.go -only <agent>`, then ")
	b.WriteString("`go test ./internal/agent -run TestRecordedFixtures -update`. Keep older versions: they are the matrix.\n\n")
	b.WriteString("The recorder refuses existing version directories; use a separate `-out` directory to review a replacement. Gemini uses the native `--skip-trust` automation flag only in its empty temporary workspace; no persistent trust record is created.\n\n")
	b.WriteString("Gemini 0.64.0 also retains `provider-roundtrip.json`: `-only gemini-cli -gemini-provider-roundtrip -out <temporary-directory>` verifies saved-token and fixed GEMINI_API_KEY GenerateContent apply/clear through the real CLI, with no endpoint/model/key flags masking persisted settings. The fake endpoint validates x-goog-api-key and native sessions record the expected applied/restored models. `TestRecordedGeminiProviderRoundTrip` replays those inputs. Original session fixtures stay retained; trust is supplied only by the native ephemeral --skip-trust flag inside an empty temporary workspace.\n\n")
	b.WriteString("OpenCode 2.0.32 also retains `provider-roundtrip.json`: `-only opencode -opencode-provider-roundtrip -out <temporary-directory>` records token, environment-reference and keyless Chat Completions apply/clear with synthetic credentials. Each config is verified by native `run --standalone` and its recorded model; the original session fixture is retained. `TestRecordedOpenCodeProviderRoundTrip` replays those edits. A background service may cache old settings; reload/restart it before testing changed global settings. JSONC comments/trailing commas and multiple global files are not supported for provider writes.\n\n")
	b.WriteString("| Agent | Version | Recorded | OS/architecture | How |\n|---|---|---|---|---|\n")
	for _, o := range origins {
		platform := "unknown"
		if o.OS != "" {
			platform = o.OS + "/" + o.Arch
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", o.Agent, o.Version, o.Recorded, platform, o.How)
	}
	return b.String()
}
