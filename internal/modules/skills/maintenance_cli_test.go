package skills

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func maintenanceOutput(t *testing.T, svc *Service, agents []agent.Agent, args []string, dst any) {
	t.Helper()
	out, stderr, code := run(t, append([]string{"skills"}, args...), svc, agents)
	if code != 0 {
		t.Fatalf("%v: exit %d: %s\n%s", args, code, stderr, out)
	}
	if dst != nil {
		if err := json.Unmarshal([]byte(out), dst); err != nil {
			t.Fatalf("%v: invalid JSON: %s %v", args, out, err)
		}
	}
}

func TestCLISourcesAreReadOnlyAndUseSavedRefScope(t *testing.T) {
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	writeSkill(t, repo, "scope/one", validMD("same", "one"))
	writeSkill(t, repo, "scope/two", validMD("same", "two"))
	writeSkill(t, repo, "outside", validMD("outside", "outside"))
	git("add", ".")
	git("commit", "-m", "source")
	git("tag", "stable")
	svc := New(testPaths(t))
	r := repository{Name: "Fixture", URL: localRepositoryURL(repo), Ref: "stable", Subdir: "scope"}
	if err := svc.saveRepository(r, nil, false); err != nil {
		t.Fatal(err)
	}
	config, _ := os.ReadFile(svc.paths.ConfigPath())
	var sources []repository
	maintenanceOutput(t, svc, nil, []string{"sources", "list", "--json"}, &sources)
	if !reflect.DeepEqual(sources, []repository{r}) {
		t.Fatalf("source list: %+v", sources)
	}
	var out jsonSourceScan
	maintenanceOutput(t, svc, nil, []string{"sources", "scan", "--json", "Fixture"}, &out)
	if out.Revision == "" || len(out.Entries) != 2 || out.Entries[0].Path != "scope/one" || out.Entries[1].Path != "scope/two" || !out.Entries[0].Collision || out.Entries[0].Kind != "skill" {
		t.Fatalf("source identity/collisions lost: %+v", out)
	}
	after, _ := os.ReadFile(svc.paths.ConfigPath())
	if !bytes.Equal(config, after) {
		t.Fatal("source queries rewrote config")
	}
	if skills, _ := svc.Scan(nil); len(skills) != 0 {
		t.Fatal("scan installed skills")
	}
	if _, err := os.Stat(svc.paths.DataDir); !os.IsNotExist(err) {
		t.Fatal("source queries created app data")
	}
	// Listing and argument rejection must not run detection or Git.
	t.Setenv("PATH", "")
	c := cli.Context{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Paths: svc.paths, Agents: func() []agent.Agent { t.Fatal("read-only source query detected agents"); return nil }}
	for _, args := range [][]string{{"sources", "list"}, {"sources", "scan", "missing"}, {"sources", "scan", "Fixture", "extra"}} {
		code := cli.Run(append([]string{"skills"}, args...), c, commands(svc))
		if (len(args) == 2 && code != 0) || (len(args) > 2 && code != 1) {
			t.Fatalf("unexpected status: %v %d", args, code)
		}
	}
}

func TestCLIUpdateCheckSelectedApplyAndProfilesMatchServices(t *testing.T) {
	svc, _, _, _ := reviewFixture(t)
	agents := []agent.Agent{testAgent(svc.paths.Home, "claude-code", "claude/skills"), testAgent(svc.paths.Home, "codex", "codex/skills")}
	// Save a managed matrix, then change it so profile preview has real deltas.
	sk := scanOne(t, svc, agents, "alpha")
	if err := svc.Enable(sk, agents[0], agents); err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	maintenanceOutput(t, svc, agents, []string{"profiles", "save", "work", "--json"}, &saved)
	if err := svc.Disable(scanOne(t, svc, agents, "alpha"), agents[0], agents); err != nil {
		t.Fatal(err)
	}
	var review jsonUpdateReview
	maintenanceOutput(t, svc, agents, []string{"updates", "check", "--json"}, &review)
	if len(review.Items) != 2 || review.Review == "" || review.Items[0].Status != "available" || review.Items[0].Files == nil || review.Items[0].LocalChanged == nil || *review.Items[0].LocalChanged || review.Items[0].UpstreamChanged == nil || !*review.Items[0].UpstreamChanged || !strings.Contains(review.Items[0].Diff, "+description: version two") {
		t.Fatalf("missing review evidence: %+v", review)
	}
	var result jsonMaintenanceResult
	maintenanceOutput(t, svc, agents, []string{"updates", "apply", "alpha", "--review", review.Items[0].Review, "--json"}, &result)
	if len(result.Targets) != 1 || result.Targets[0].ID != "alpha / library" || result.Targets[0].Outcome != core.Succeeded {
		t.Fatalf("wrong update result: %+v", result)
	}
	if scanOne(t, svc, nil, "alpha").Description != "version two" || scanOne(t, svc, nil, "beta").Description != "version one" {
		t.Fatal("selected update expanded its scope")
	}
	var preview jsonProfilePreview
	maintenanceOutput(t, svc, agents, []string{"profiles", "preview", "work", "--json"}, &preview)
	serviceDiff, err := svc.DiffProfile("work", agents)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 1 || len(preview.Changes) != len(serviceDiff) || preview.Changes[0].Dir != "alpha" || !reflect.DeepEqual(preview.Changes[0].Add, serviceDiff[0].Add) || !reflect.DeepEqual(preview.Targets, []string{"claude-code", "codex"}) {
		t.Fatalf("profile preview diverged: %+v", preview)
	}
	maintenanceOutput(t, svc, agents, []string{"profiles", "apply", "work", "--review", preview.Review, "--json"}, &result)
	actual, _ := svc.Scan(agents)
	spec, _ := svc.GetProfile("work")
	if !reflect.DeepEqual(BuildProfileSpec(actual, agents), spec) || result.Targets[0].Outcome != core.Succeeded {
		t.Fatalf("profile application diverged: %+v", result)
	}
	backups, _ := svc.ListBackups()
	if len(backups) != 1 || backups[0].SkillDir != "alpha" {
		t.Fatal("selected update backup missing")
	}
}

func TestCLIUpdateReviewRefusesStaleScopeAndProtectsLocalEdits(t *testing.T) {
	for _, change := range []string{"selection", "local", "upstream", "origin", "library", "replace-many"} {
		t.Run(change, func(t *testing.T) {
			svc, repo, git, _ := reviewFixture(t)
			var review jsonUpdateReview
			maintenanceOutput(t, svc, nil, []string{"updates", "check", "alpha", "--json"}, &review)
			names := []string{"alpha"}
			token := review.Review
			extra := []string{}
			switch change {
			case "selection":
				names = []string{"alpha", "beta"}
			case "local":
				writeSkill(t, svc.paths.LibraryDir(), "alpha", validMD("alpha", "local edit"))
			case "upstream":
				writeSkill(t, repo, "alpha", validMD("alpha", "upstream three"))
				git("add", ".")
				git("commit", "-m", "three")
			case "origin":
				path := filepath.Join(svc.paths.LibraryDir(), "alpha")
				o := *readOrigin(path)
				o.Hash = "changed"
				if err := writeOrigin(path, o); err != nil {
					t.Fatal(err)
				}
			case "library":
				other := New(testPaths(t))
				installGitSkill(t, other, localRepositoryURL(repo), "alpha", nil)
				svc = other
			case "replace-many":
				names = []string{"alpha", "beta"}
				extra = []string{"--replace-local"}
			}
			before, _ := hashDir(svc.paths.LibraryDir())
			args := append([]string{"skills", "updates", "apply"}, names...)
			args = append(args, "--review", token)
			args = append(args, extra...)
			if _, stderr, code := run(t, args, svc, nil); code != 1 {
				t.Fatalf("stale apply accepted: %s", stderr)
			}
			after, _ := hashDir(svc.paths.LibraryDir())
			backups, _ := svc.ListBackups()
			if before != after || len(backups) != 0 {
				t.Fatal("invalid review wrote files")
			}
		})
	}
	t.Run("single explicit overwrite and bulk preservation", func(t *testing.T) {
		svc, _, _, _ := reviewFixture(t)
		writeSkill(t, svc.paths.LibraryDir(), "alpha", validMD("alpha", "edited locally"))
		var review jsonUpdateReview
		maintenanceOutput(t, svc, nil, []string{"updates", "check", "--json"}, &review)
		if review.Items[0].Status != "locally_edited" {
			t.Fatal("local changes not identified")
		}
		var result jsonMaintenanceResult
		maintenanceOutput(t, svc, nil, []string{"updates", "apply", "alpha", "beta", "--review", review.Review, "--json"}, &result)
		if result.Targets[0].Outcome != core.Skipped || scanOne(t, svc, nil, "alpha").Description != "edited locally" {
			t.Fatal("bulk overwrote local edits")
		}
		maintenanceOutput(t, svc, nil, []string{"updates", "apply", "alpha", "--review", review.Items[0].Review, "--replace-local", "--json"}, &result)
		if result.Targets[0].Outcome != core.Succeeded || scanOne(t, svc, nil, "alpha").Description != "version two" {
			t.Fatal("explicit replacement failed")
		}
	})
}

func TestCLIMaintenanceRejectsInvalidArgumentsBeforeWritesOrFetch(t *testing.T) {
	svc, _, _, _ := reviewFixture(t)
	writeSkill(t, svc.paths.LibraryDir(), "display-folder", validMD("alias", "manual"))
	t.Setenv("PATH", "")
	before, _ := hashDir(svc.paths.LibraryDir())
	for _, args := range [][]string{
		{"updates", "check", "alpha", "unknown"}, {"updates", "check", "alpha", "alpha"},
		{"updates", "check", "alias"}, {"updates", "check", "display-folder"},
		{"updates", "apply", "alpha"}, {"updates", "apply", "alpha", "--review", "invalid"},
		{"updates", "check", "--replace-local"}, {"updates", "check", "--review", strings.Repeat("a", 64)},
		{"profiles", "save", "name", "extra"}, {"profiles", "remove", "name", "extra"},
		{"profiles", "apply", "name", "--review", "invalid"}, {"profiles", "list", "extra"},
		{"sources", "scan", "name", "extra"}, {"sources", "list", "extra"},
	} {
		out, stderr, code := run(t, append([]string{"skills"}, args...), svc, nil)
		if code != 1 || strings.Contains(stderr, "requires git") || strings.Contains(stderr, "git clone") {
			t.Fatalf("bad selection fetched or passed: %v %d %s %s", args, code, out, stderr)
		}
	}
	after, _ := hashDir(svc.paths.LibraryDir())
	if before != after {
		t.Fatal("bad arguments wrote library")
	}
	for _, path := range []string{svc.paths.BackupsDir(), svc.paths.ProfilesPath(), svc.paths.ConfigPath()} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("bad arguments wrote %s", path)
		}
	}
}

func TestCLIProfileStableCanonicalPreviewAndStaleGuards(t *testing.T) {
	for _, change := range []string{"profile", "matrix", "destinations"} {
		t.Run(change, func(t *testing.T) {
			svc, agents := testSkillSvc(t)
			writeSkill(t, svc.paths.LibraryDir(), "one", validMD("same", "one"))
			writeSkill(t, svc.paths.LibraryDir(), "two", validMD("same", "two"))
			if err := svc.SaveProfile("work", ProfileSpec{"one": {"claude-code"}, "two": {"claude-code"}}); err != nil {
				t.Fatal(err)
			}
			var first jsonProfilePreview
			maintenanceOutput(t, svc, agents, []string{"profiles", "preview", "work", "--json"}, &first)
			if len(first.Changes) != 2 || first.Changes[0].Dir != "one" || first.Changes[1].Dir != "two" {
				t.Fatalf("display aliases lost identity: %+v", first)
			}
			for range 10 {
				var next jsonProfilePreview
				maintenanceOutput(t, svc, agents, []string{"profiles", "preview", "work", "--json"}, &next)
				if first.Review != next.Review || !reflect.DeepEqual(first.Changes, next.Changes) {
					t.Fatal("preview is nondeterministic")
				}
			}
			switch change {
			case "profile":
				if err := svc.SaveProfile("work", ProfileSpec{}); err != nil {
					t.Fatal(err)
				}
			case "matrix":
				if err := svc.Enable(scanOne(t, svc, agents, "one"), agents[0], agents); err != nil {
					t.Fatal(err)
				}
			case "destinations":
				agents[0].ManagedDir = filepath.Join(svc.paths.Home, "another")
				agents[0].ReadDirs = []string{agents[0].ManagedDir}
			}
			before, _ := svc.Scan(agents)
			spec := BuildProfileSpec(before, agents)
			out, stderr, code := run(t, []string{"skills", "profiles", "apply", "work", "--review", first.Review, "--json"}, svc, agents)
			var result jsonMaintenanceResult
			if code != 1 || json.Unmarshal([]byte(out), &result) != nil || result.Error == "" {
				t.Fatalf("stale profile accepted: %s %s", out, stderr)
			}
			after, _ := svc.Scan(agents)
			if !reflect.DeepEqual(spec, BuildProfileSpec(after, agents)) {
				t.Fatal("stale preview changed activation")
			}
		})
	}
}

func TestCLIProfilesPreserveLegacyAndUnknownFields(t *testing.T) {
	svc, agents := testSkillSvc(t)
	writeSkillLib(t, svc, "one", "one")
	if err := os.MkdirAll(filepath.Dir(svc.paths.ProfilesPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.paths.ProfilesPath(), []byte(`{"foreign":{"keep":true},"profiles":{"legacy":["one"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var names []string
	maintenanceOutput(t, svc, agents, []string{"profiles", "list", "--json"}, &names)
	if !reflect.DeepEqual(names, []string{"legacy"}) {
		t.Fatal(names)
	}
	var shown struct {
		Name string      `json:"name"`
		Spec ProfileSpec `json:"spec"`
	}
	maintenanceOutput(t, svc, agents, []string{"profiles", "show", "legacy", "--json"}, &shown)
	if !slices.Equal(shown.Spec["one"], []string{"*"}) {
		t.Fatal(shown)
	}
	var preview jsonProfilePreview
	maintenanceOutput(t, svc, agents, []string{"profiles", "preview", "legacy", "--json"}, &preview)
	maintenanceOutput(t, svc, agents, []string{"profiles", "apply", "legacy", "--review", preview.Review, "--json"}, nil)
	maintenanceOutput(t, svc, agents, []string{"profiles", "save", "saved", "--json"}, nil)
	maintenanceOutput(t, svc, agents, []string{"profiles", "remove", "saved", "--json"}, nil)
	maintenanceOutput(t, svc, agents, []string{"profiles", "remove", "saved", "--json"}, nil)
	raw, _, err := svc.readProfilesRaw()
	var foreign map[string]bool
	decodeErr := json.Unmarshal(raw["foreign"], &foreign)
	if err != nil || decodeErr != nil || !foreign["keep"] {
		t.Fatalf("foreign metadata lost: %s %v", raw["foreign"], err)
	}
	if !scanOne(t, svc, agents, "one").States["claude-code"].On {
		t.Fatal("profile removal touched activation")
	}
}

func TestCLIProfilePartialFailureKeepsStructuredTargetResults(t *testing.T) {
	svc, agents := testSkillSvc(t)
	other := testAgent(svc.paths.Home, "codex", "codex/skills")
	agents = append(agents, other)
	writeSkillLib(t, svc, "one", "one")
	if err := os.MkdirAll(filepath.Dir(other.ManagedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other.ManagedDir, []byte("foreign obstruction"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveProfile("work", ProfileSpec{"one": {"claude-code", "codex"}}); err != nil {
		t.Fatal(err)
	}
	var preview jsonProfilePreview
	maintenanceOutput(t, svc, agents, []string{"profiles", "preview", "work", "--json"}, &preview)
	out, stderr, code := run(t, []string{"skills", "profiles", "apply", "work", "--review", preview.Review, "--json"}, svc, agents)
	var result jsonMaintenanceResult
	if code != 1 || json.Unmarshal([]byte(out), &result) != nil || len(result.Targets) != 2 || result.Targets[0].Outcome != core.Succeeded || result.Targets[1].Outcome != core.Failed || !strings.Contains(stderr, "Preview the skill profile again") {
		t.Fatalf("partial results lost: %s %s", out, stderr)
	}
	if !scanOne(t, svc, agents, "one").States["claude-code"].On {
		t.Fatal("successful target was lost")
	}
	if data, _ := os.ReadFile(other.ManagedDir); string(data) != "foreign obstruction" {
		t.Fatal("foreign obstruction overwritten")
	}
}
