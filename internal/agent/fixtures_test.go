package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
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
			got := goldenFor(t, ad)
			data, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, '\n')
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
		out = append(out, o)
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

func goldenFor(t *testing.T, ad Adapter) fixtureGolden {
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
				gs.Usage = &u
			}
		}
		if er, ok := ad.(UsageEventReader); ok {
			events, err := er.UsageEvents(s)
			if err != nil {
				t.Fatalf("UsageEvents(%s): %v", s.ID, err)
			}
			if len(events) > 0 {
				ev := &goldenEv{}
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
					if !slices.Contains(ev.Models, e.Model) {
						ev.Models = append(ev.Models, e.Model)
					}
				}
				gs.Events = ev
			}
		}
		g.Sessions = append(g.Sessions, gs)
	}
	// Claude Code limits come from the network, never from a fixture.
	if rl, ok := ad.(RateLimitReader); ok && ad.ID() != "claude-code" {
		if st, err := rl.RateLimits(context.Background()); err == nil {
			st.FetchedAt = st.FetchedAt.UTC() // the golden must not depend on the machine's zone
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
	b.WriteString("`scripts/record-fixtures.go` records them in a throwaway home against a local fake model: no account, no network, ")
	b.WriteString("paths replaced by `/work/proj` and `/home/user`, long strings (system prompts) cut, and a recording that still names the machine fails.\n\n")
	b.WriteString("To pin a new CLI release, install it and run `go run scripts/record-fixtures.go -only <agent>`, then ")
	b.WriteString("`go test ./internal/agent -run TestRecordedFixtures -update`. Keep older versions: they are the matrix.\n\n")
	b.WriteString("| Agent | Version | Recorded | How |\n|---|---|---|---|\n")
	for _, o := range origins {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", o.Agent, o.Version, o.Recorded, o.How)
	}
	return b.String()
}
